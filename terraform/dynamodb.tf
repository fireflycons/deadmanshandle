# The handle's mutable state: the timeout, the delivery state and the
# recorded document ETag, in one item. Kept apart from the config parameter
# so that concurrent Lambdas can update it atomically, with conditions that
# Parameter Store does not offer.
resource "aws_dynamodb_table" "state" {
  name         = "${var.application_name}-state"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "id"

  attribute {
    name = "id"
    type = "S"
  }

  # The timeout is the one thing that must not be lost
  point_in_time_recovery {
    enabled = true
  }

  tags = merge(var.tags, {
    Name = "Handle State"
  })
}

# Seeded with the config file's timeout. After that the Lambdas own the item
# (each check-in rewrites the timeout), so later applies must not reset it.
# To re-seed deliberately, run:
#   terraform apply -replace=aws_dynamodb_table_item.state
resource "aws_dynamodb_table_item" "state" {
  table_name = aws_dynamodb_table.state.name
  hash_key   = aws_dynamodb_table.state.hash_key

  # The id must match stateItemID in internal/adapters/dynamodb_state_store.go
  item = jsonencode({
    id      = { S = "handle" }
    timeout = { S = try(local.seed_config.timeout, "") }
  })

  lifecycle {
    ignore_changes = [item]

    precondition {
      condition     = try(timecmp(local.seed_config.timeout, "0001-01-01T00:00:00Z") > 0, false)
      error_message = "Config: timeout is missing or not an RFC 3339 time."
    }
  }
}
