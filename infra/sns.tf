resource "aws_sns_topic" "events" {
  name = "sodatracker-events-${var.environment}"
}
