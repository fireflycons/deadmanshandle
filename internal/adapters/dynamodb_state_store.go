package adapters

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// stateItemID is the key of the single item holding the state. Terraform
// creates the item (see terraform/dynamodb.tf).
const stateItemID = "handle"

// Attribute names, always passed as expression attribute names so that
// DynamoDB reserved words cannot clash with them
var stateAttributeNames = map[string]string{
	"#t": "timeout",
	"#c": "checkIns",
	"#s": "sentTo",
	"#o": "ownerNotified",
	"#e": "documentETag",
}

// DynamoDBStateStore implements the StateStore interface with one DynamoDB item
type DynamoDBStateStore struct {
	client    *dynamodb.Client
	tableName string
}

// NewDynamoDBStateStore creates a new DynamoDB-based state store
func NewDynamoDBStateStore(client *dynamodb.Client, tableName string) *DynamoDBStateStore {
	return &DynamoDBStateStore{
		client:    client,
		tableName: tableName,
	}
}

// GetState reads the state with a strongly consistent read, so it reflects
// every update that has completed
func (s *DynamoDBStateStore) GetState(ctx context.Context) (*config.State, error) {
	output, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      &s.tableName,
		Key:            stateKey(),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	if output.Item == nil {
		return nil, fmt.Errorf("state item %q is missing from table %s", stateItemID, s.tableName)
	}

	var state config.State
	timeout, ok := output.Item["timeout"].(*types.AttributeValueMemberS)
	if !ok {
		return nil, errors.New("state has no timeout")
	}
	if state.Timeout, err = time.Parse(time.RFC3339, timeout.Value); err != nil {
		return nil, fmt.Errorf("state timeout: %w", err)
	}
	if checkIns, ok := output.Item["checkIns"].(*types.AttributeValueMemberN); ok {
		if state.CheckIns, err = strconv.ParseInt(checkIns.Value, 10, 64); err != nil {
			return nil, fmt.Errorf("state checkIns: %w", err)
		}
	}
	if sentTo, ok := output.Item["sentTo"].(*types.AttributeValueMemberSS); ok {
		state.SentTo = sentTo.Value
	}
	if notified, ok := output.Item["ownerNotified"].(*types.AttributeValueMemberBOOL); ok {
		state.OwnerNotified = notified.Value
	}
	if etag, ok := output.Item["documentETag"].(*types.AttributeValueMemberS); ok {
		state.DocumentETag = etag.Value
	}
	return &state, nil
}

// CheckIn sets the timeout and clears the delivery state in one update
func (s *DynamoDBStateStore) CheckIn(ctx context.Context, timeout time.Time) error {
	return s.update(ctx, "SET #t = :t REMOVE #s, #o ADD #c :one", "", map[string]types.AttributeValue{
		":t":   &types.AttributeValueMemberS{Value: timeout.UTC().Format(time.RFC3339)},
		":one": &types.AttributeValueMemberN{Value: "1"},
	})
}

// RecordSent adds recipient to the sentTo set, unless there was a check-in
func (s *DynamoDBStateStore) RecordSent(ctx context.Context, checkIns int64, recipient string) error {
	return s.update(ctx, "ADD #s :r", checkInsUnchanged(checkIns), map[string]types.AttributeValue{
		":r": &types.AttributeValueMemberSS{Value: []string{recipient}},
		":c": checkInsValue(checkIns),
	})
}

// RecordOwnerNotified sets ownerNotified, unless there was a check-in
func (s *DynamoDBStateStore) RecordOwnerNotified(ctx context.Context, checkIns int64) error {
	return s.update(ctx, "SET #o = :true", checkInsUnchanged(checkIns), map[string]types.AttributeValue{
		":true": &types.AttributeValueMemberBOOL{Value: true},
		":c":    checkInsValue(checkIns),
	})
}

// SwapDocumentETag records current if the recorded ETag is still previous
func (s *DynamoDBStateStore) SwapDocumentETag(ctx context.Context, previous, current string) error {
	values := map[string]types.AttributeValue{
		":n": &types.AttributeValueMemberS{Value: current},
	}
	condition := "attribute_exists(#t) AND attribute_not_exists(#e)"
	if previous != "" {
		condition = "#e = :p"
		values[":p"] = &types.AttributeValueMemberS{Value: previous}
	}
	return s.update(ctx, "SET #e = :n", condition, values)
}

// update applies an update expression to the state item, returning
// ports.ErrConditionFailed if condition is false
func (s *DynamoDBStateStore) update(ctx context.Context, expression, condition string, values map[string]types.AttributeValue) error {
	input := &dynamodb.UpdateItemInput{
		TableName:                 &s.tableName,
		Key:                       stateKey(),
		UpdateExpression:          &expression,
		ExpressionAttributeNames:  usedNames(expression + " " + condition),
		ExpressionAttributeValues: values,
	}
	if condition != "" {
		input.ConditionExpression = &condition
	}

	_, err := s.client.UpdateItem(ctx, input)
	var failed *types.ConditionalCheckFailedException
	if errors.As(err, &failed) {
		return ports.ErrConditionFailed
	}
	return err
}

// checkInsUnchanged is the condition that no check-in happened since the
// state was read. checkIns is absent on an item Terraform has just seeded.
// attribute_exists(#t) stops an update from creating a missing item.
func checkInsUnchanged(checkIns int64) string {
	if checkIns == 0 {
		return "attribute_exists(#t) AND (attribute_not_exists(#c) OR #c = :c)"
	}
	return "#c = :c"
}

func checkInsValue(checkIns int64) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.FormatInt(checkIns, 10)}
}

func stateKey() map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"id": &types.AttributeValueMemberS{Value: stateItemID},
	}
}

// usedNames returns the expression attribute names that appear in
// expression, since DynamoDB rejects unused ones. No placeholder is a prefix
// of another, so a substring match is enough.
func usedNames(expression string) map[string]string {
	names := make(map[string]string)
	for placeholder, name := range stateAttributeNames {
		if strings.Contains(expression, placeholder) {
			names[placeholder] = name
		}
	}
	return names
}
