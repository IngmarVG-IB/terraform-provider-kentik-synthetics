data "kentik-synthetics_test" "existing" {
  id = "3271"
}

output "existing_test_status" {
  value = data.kentik-synthetics_test.existing.status
}
