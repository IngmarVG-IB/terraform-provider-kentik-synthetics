# Page load test with headless Chromium (requires an app agent).
resource "kentik-synthetics_test" "page_load" {
  name = "example-page-load"
  type = "page_load"

  settings {
    page_load {
      target  = "https://www.example.com/"
      timeout = 10000
      css_selectors = {
        "logo" = "#logo"
      }
    }
    agent_ids = ["1717"]
    tasks     = ["page-load"]
  }
}
