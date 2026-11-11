data "kentik-synthetics_tests" "all" {}

output "paused_test_names" {
  value = [for t in data.kentik-synthetics_tests.all.items : t.name if t.status == "TEST_STATUS_PAUSED"]
}
