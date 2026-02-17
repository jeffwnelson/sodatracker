resource "aws_lambda_function" "api" {
  function_name = "sodatracker-api-${var.environment}"
  role          = aws_iam_role.lambda_exec.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"]
  memory_size   = 256
  timeout       = 30

  filename         = "${path.module}/../bin/lambda.zip"
  source_code_hash = filebase64sha256("${path.module}/../bin/lambda.zip")

  environment {
    variables = {
      TABLE_NAME     = aws_dynamodb_table.main.name
      API_KEY        = var.api_key
      JWT_SECRET     = var.jwt_secret
      ADMIN_PASSWORD = var.admin_password
      SNS_TOPIC_ARN  = aws_sns_topic.events.arn
    }
  }
}

resource "aws_lambda_permission" "apigw" {
  statement_id  = "AllowAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.api.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.main.execution_arn}/*/*"
}

resource "aws_cloudwatch_log_group" "lambda" {
  name              = "/aws/lambda/${aws_lambda_function.api.function_name}"
  retention_in_days = 14
}
