// Package fakesynthetics is an in-memory implementation of kentik.synthetics.v202309.SyntheticsAdminService
// served over gRPC bufconn. Unit tests use it instead of a live Kentik tenant.
//
// The fake checks that both authentication headers are present, keeps tests, agents and agent alerts in
// memory, fills a few server-side defaults like the real API, records every call and can inject errors.
// To support a new RPC or service, implement the method on Server (or add a sibling fake package) and
// register it in Start.
package fakesynthetics

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	corepb "github.com/kentik/api-schema-public/gen/go/kentik/core/v202303"
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Fake test identity used for created_by / last_updated_by.
const (
	FakeUserID    = "4242"
	FakeUserEmail = "fake.user@example.com"
	FakeUserName  = "Fake User"
)

// Call records one RPC received by the fake.
type Call struct {
	Method string
	Email  string
	Token  string
}

// Server is the fake SyntheticsAdminService. Use Start to run it.
type Server struct {
	syntheticspb.UnimplementedSyntheticsAdminServiceServer

	mu       sync.Mutex
	nextID   int
	tests    map[string]*syntheticspb.Test
	agents   map[string]*syntheticspb.Agent
	alerts   map[string]*syntheticspb.AgentAlert
	calls    []Call
	failures map[string][]error

	lis *bufconn.Listener
	srv *grpc.Server
}

// New returns an empty fake that is not yet serving.
func New() *Server {
	return &Server{
		nextID:   1000,
		tests:    map[string]*syntheticspb.Test{},
		agents:   map[string]*syntheticspb.Agent{},
		alerts:   map[string]*syntheticspb.AgentAlert{},
		failures: map[string][]error{},
	}
}

// Start serves a new fake on a bufconn listener and stops it when the test ends.
func Start(t testing.TB) *Server {
	t.Helper()
	s := New()
	s.Serve()
	t.Cleanup(s.Stop)
	return s
}

// Serve starts serving on an in-memory listener.
func (s *Server) Serve() {
	s.lis = bufconn.Listen(1 << 20)
	s.srv = grpc.NewServer(grpc.UnaryInterceptor(s.intercept))
	syntheticspb.RegisterSyntheticsAdminServiceServer(s.srv, s)
	go func() { _ = s.srv.Serve(s.lis) }()
}

// Stop stops the server.
func (s *Server) Stop() {
	if s.srv != nil {
		s.srv.Stop()
	}
}

// Endpoint is a placeholder address; DialOptions route it to the in-memory listener.
func (s *Server) Endpoint() string { return "passthrough:///bufnet" }

// DialOptions returns the options needed to reach the fake (bufconn dialer, plaintext).
func (s *Server) DialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return s.lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
}

// FailNext makes the next len(errs) calls of method (e.g. "GetTest") return the given errors in order.
func (s *Server) FailNext(method string, errs ...error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures[method] = append(s.failures[method], errs...)
}

// Calls returns a copy of the recorded calls.
func (s *Server) Calls() []Call {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls)
}

// CallCount returns how many times method was called.
func (s *Server) CallCount(method string) int {
	n := 0
	for _, c := range s.Calls() {
		if c.Method == method {
			n++
		}
	}
	return n
}

// AddAgent seeds an agent. The agent is stored as a copy.
func (s *Server) AddAgent(a *syntheticspb.Agent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents[a.GetId()] = clone(a)
}

// AddTest seeds a test and returns its ID (generated when empty).
func (s *Server) AddTest(t *syntheticspb.Test) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := clone(t)
	if c.GetId() == "" {
		c.Id = s.newID()
	}
	s.tests[c.GetId()] = c
	return c.GetId()
}

// AddAgentAlert seeds an agent alert and returns its ID (generated when empty).
func (s *Server) AddAgentAlert(a *syntheticspb.AgentAlert) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := clone(a)
	if c.GetId() == "" {
		c.Id = s.newID()
	}
	s.alerts[c.GetId()] = c
	return c.GetId()
}

// Test returns a copy of the stored test, or nil.
func (s *Server) Test(id string) *syntheticspb.Test {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneOrNil(s.tests[id])
}

// Agent returns a copy of the stored agent, or nil.
func (s *Server) Agent(id string) *syntheticspb.Agent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneOrNil(s.agents[id])
}

// AgentAlert returns a copy of the stored agent alert, or nil.
func (s *Server) AgentAlert(id string) *syntheticspb.AgentAlert {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneOrNil(s.alerts[id])
}

func (s *Server) intercept(
	ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
) (any, error) {
	method := info.FullMethod[len("/kentik.synthetics.v202309.SyntheticsAdminService/"):]
	md, _ := metadata.FromIncomingContext(ctx)
	email := first(md.Get("x-ch-auth-email"))
	token := first(md.Get("x-ch-auth-api-token"))

	s.mu.Lock()
	s.calls = append(s.calls, Call{Method: method, Email: email, Token: token})
	var injected error
	if errs := s.failures[method]; len(errs) > 0 {
		injected, s.failures[method] = errs[0], errs[1:]
	}
	s.mu.Unlock()

	if email == "" || token == "" {
		return nil, status.Error(codes.Unauthenticated, "missing X-CH-Auth-Email or X-CH-Auth-API-Token")
	}
	if injected != nil {
		return nil, injected
	}
	return handler(ctx, req)
}

// --- agents ---

func (s *Server) ListAgents(context.Context, *syntheticspb.ListAgentsRequest) (*syntheticspb.ListAgentsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := &syntheticspb.ListAgentsResponse{}
	for _, id := range sortedKeys(s.agents) {
		resp.Agents = append(resp.Agents, clone(s.agents[id]))
	}
	return resp, nil
}

func (s *Server) GetAgent(_ context.Context, r *syntheticspb.GetAgentRequest) (*syntheticspb.GetAgentResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.agents[r.GetId()]
	if !ok {
		return nil, notFound("agent", r.GetId())
	}
	return &syntheticspb.GetAgentResponse{Agent: clone(a)}, nil
}

func (s *Server) UpdateAgent(_ context.Context, r *syntheticspb.UpdateAgentRequest) (*syntheticspb.UpdateAgentResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	in := r.GetAgent()
	cur, ok := s.agents[in.GetId()]
	if !ok {
		return nil, notFound("agent", in.GetId())
	}
	next := clone(in)
	// Output-only fields are kept from the stored agent.
	next.Type, next.Os, next.Version = cur.GetType(), cur.GetOs(), cur.GetVersion()
	next.Ip = cur.GetIp() //nolint:staticcheck // deprecated, but still output-only
	next.LastAuthed, next.TestIds, next.AgentImpl = cur.GetLastAuthed(), cur.GetTestIds(), cur.GetAgentImpl()
	if next.GetStatus() == syntheticspb.AgentStatus_AGENT_STATUS_UNSPECIFIED {
		next.Status = cur.GetStatus()
	}
	s.agents[in.GetId()] = next
	return &syntheticspb.UpdateAgentResponse{Agent: clone(next)}, nil
}

func (s *Server) DeleteAgent(_ context.Context, r *syntheticspb.DeleteAgentRequest) (*syntheticspb.DeleteAgentResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.agents[r.GetId()]; !ok {
		return nil, notFound("agent", r.GetId())
	}
	delete(s.agents, r.GetId())
	return &syntheticspb.DeleteAgentResponse{}, nil
}

// --- tests ---

func (s *Server) ListTests(context.Context, *syntheticspb.ListTestsRequest) (*syntheticspb.ListTestsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := &syntheticspb.ListTestsResponse{}
	for _, id := range sortedKeys(s.tests) {
		resp.Tests = append(resp.Tests, clone(s.tests[id]))
	}
	return resp, nil
}

func (s *Server) CreateTest(_ context.Context, r *syntheticspb.CreateTestRequest) (*syntheticspb.CreateTestResponse, error) {
	t := clone(r.GetTest())
	if err := validateTest(t); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t.Id = s.newID()
	now := timestamppb.New(time.Now().UTC().Truncate(time.Second))
	t.Cdate, t.Edate = now, now
	t.CreatedBy = fakeUser()
	t.LastUpdatedBy = fakeUser()
	if t.GetStatus() == syntheticspb.TestStatus_TEST_STATUS_UNSPECIFIED {
		t.Status = syntheticspb.TestStatus_TEST_STATUS_ACTIVE
	}
	applyDefaults(t)
	s.tests[t.GetId()] = t
	return &syntheticspb.CreateTestResponse{Test: clone(t)}, nil
}

func (s *Server) GetTest(_ context.Context, r *syntheticspb.GetTestRequest) (*syntheticspb.GetTestResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tests[r.GetId()]
	if !ok {
		return nil, notFound("test", r.GetId())
	}
	return &syntheticspb.GetTestResponse{Test: clone(t)}, nil
}

// UpdateTest replaces the test configuration. Like the real API it does not change the status; use
// SetTestStatus for that.
func (s *Server) UpdateTest(_ context.Context, r *syntheticspb.UpdateTestRequest) (*syntheticspb.UpdateTestResponse, error) {
	in := clone(r.GetTest())
	if err := validateTest(in); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.tests[in.GetId()]
	if !ok {
		return nil, notFound("test", in.GetId())
	}
	if in.GetType() != cur.GetType() {
		return nil, status.Errorf(codes.InvalidArgument, "test type cannot be changed (%q -> %q)", cur.GetType(), in.GetType())
	}
	in.Status = cur.GetStatus()
	in.Cdate, in.CreatedBy = cur.GetCdate(), cur.GetCreatedBy()
	in.Edate = timestamppb.New(time.Now().UTC().Truncate(time.Second))
	in.LastUpdatedBy = fakeUser()
	applyDefaults(in)
	s.tests[in.GetId()] = in
	return &syntheticspb.UpdateTestResponse{Test: clone(in)}, nil
}

func (s *Server) DeleteTest(_ context.Context, r *syntheticspb.DeleteTestRequest) (*syntheticspb.DeleteTestResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tests[r.GetId()]; !ok {
		return nil, notFound("test", r.GetId())
	}
	delete(s.tests, r.GetId())
	return &syntheticspb.DeleteTestResponse{}, nil
}

func (s *Server) SetTestStatus(_ context.Context, r *syntheticspb.SetTestStatusRequest) (*syntheticspb.SetTestStatusResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tests[r.GetId()]
	if !ok {
		return nil, notFound("test", r.GetId())
	}
	switch r.GetStatus() {
	case syntheticspb.TestStatus_TEST_STATUS_ACTIVE, syntheticspb.TestStatus_TEST_STATUS_PAUSED:
		t.Status = r.GetStatus()
	default:
		return nil, status.Errorf(codes.InvalidArgument, "status %v is not user settable", r.GetStatus())
	}
	return &syntheticspb.SetTestStatusResponse{}, nil
}

// --- agent alerts ---

func (s *Server) CreateAgentAlert(_ context.Context, r *syntheticspb.CreateAgentAlertRequest) (*syntheticspb.CreateAgentAlertResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.agents[r.GetAgentId()]
	if !ok {
		return nil, notFound("agent", r.GetAgentId())
	}
	alert := &syntheticspb.AgentAlert{
		Id:                     s.newID(),
		ThresholdSeconds:       r.GetThresholdSeconds(),
		NotificationChannelIds: slices.Clone(r.GetNotificationChannelIds()),
		AgentId:                a.GetId(),
		AgentName:              a.GetAlias(),
	}
	s.alerts[alert.GetId()] = alert
	return &syntheticspb.CreateAgentAlertResponse{AgentAlert: clone(alert)}, nil
}

func (s *Server) UpdateAgentAlert(_ context.Context, r *syntheticspb.UpdateAgentAlertRequest) (*syntheticspb.UpdateAgentAlertResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.alerts[r.GetId()]
	if !ok {
		return nil, notFound("agent alert", r.GetId())
	}
	a.ThresholdSeconds = r.GetThresholdSeconds()
	a.NotificationChannelIds = slices.Clone(r.GetNotificationChannelIds())
	return &syntheticspb.UpdateAgentAlertResponse{AgentAlert: clone(a)}, nil
}

func (s *Server) GetAgentAlert(_ context.Context, r *syntheticspb.GetAgentAlertRequest) (*syntheticspb.GetAgentAlertResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.alerts[r.GetId()]
	if !ok {
		return nil, notFound("agent alert", r.GetId())
	}
	return &syntheticspb.GetAgentAlertResponse{AgentAlert: clone(a)}, nil
}

func (s *Server) ListAgentAlerts(_ context.Context, r *syntheticspb.ListAgentAlertsRequest) (*syntheticspb.ListAgentAlertsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := &syntheticspb.ListAgentAlertsResponse{}
	for _, id := range sortedKeys(s.alerts) {
		a := s.alerts[id]
		if len(r.GetAgentIds()) > 0 && !slices.Contains(r.GetAgentIds(), a.GetAgentId()) {
			continue
		}
		resp.AgentAlerts = append(resp.AgentAlerts, clone(a))
	}
	return resp, nil
}

func (s *Server) DeleteAgentAlert(_ context.Context, r *syntheticspb.DeleteAgentAlertRequest) (*syntheticspb.DeleteAgentAlertResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.alerts[r.GetId()]; !ok {
		return nil, notFound("agent alert", r.GetId())
	}
	delete(s.alerts, r.GetId())
	return &syntheticspb.DeleteAgentAlertResponse{}, nil
}

// --- helpers ---

func validateTest(t *syntheticspb.Test) error {
	if t.GetName() == "" {
		return status.Error(codes.InvalidArgument, "test name is required")
	}
	if t.GetSettings() == nil || len(t.GetSettings().GetAgentIds()) == 0 {
		return status.Error(codes.InvalidArgument, "settings.agent_ids is required")
	}
	want := map[string]string{
		"hostname": "hostname", "ip": "ip", "agent": "agent", "flow": "flow", "dns": "dns", "url": "url",
		"network_grid": "network_grid", "page_load": "page_load", "dns_grid": "dns_grid",
		"network_mesh": "network_mesh",
	}[t.GetType()]
	if want == "" {
		return status.Errorf(codes.InvalidArgument, "unsupported test type %q", t.GetType())
	}
	def := t.GetSettings().ProtoReflect().WhichOneof(
		t.GetSettings().ProtoReflect().Descriptor().Oneofs().ByName("definition"))
	if def == nil || string(def.Name()) != want {
		return status.Errorf(codes.InvalidArgument, "test of type %q requires settings.%s", t.GetType(), want)
	}
	return nil
}

// applyDefaults mimics server-side defaults so tests exercise Optional+Computed attributes.
func applyDefaults(t *syntheticspb.Test) {
	st := t.GetSettings()
	if st.GetPeriod() == 0 {
		st.Period = 60
	}
	if st.GetFamily() == syntheticspb.IPFamily_IP_FAMILY_UNSPECIFIED {
		st.Family = syntheticspb.IPFamily_IP_FAMILY_DUAL
	}
	if st.GetHealthSettings() == nil {
		st.HealthSettings = &syntheticspb.HealthSettings{}
	}
}

func (s *Server) newID() string {
	s.nextID++
	return strconv.Itoa(s.nextID)
}

func fakeUser() *corepb.UserInfo {
	return &corepb.UserInfo{Id: FakeUserID, Email: FakeUserEmail, FullName: FakeUserName}
}

func notFound(kind, id string) error {
	return status.Error(codes.NotFound, fmt.Sprintf("%s %q not found", kind, id))
}

func clone[T proto.Message](m T) T { return proto.Clone(m).(T) }

func cloneOrNil[T proto.Message](m T) T {
	var zero T
	if m.ProtoReflect().IsValid() {
		return clone(m)
	}
	return zero
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int {
		ai, aerr := strconv.Atoi(a)
		bi, berr := strconv.Atoi(b)
		if aerr == nil && berr == nil {
			return ai - bi
		}
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	})
	return keys
}

func first(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}
