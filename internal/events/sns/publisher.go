package sns

import (
	"context"
	"encoding/json"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/jeffwnelson/sodatracker/internal/events"
)

// Publisher forwards events to an SNS topic.
type Publisher struct {
	client   *awssns.Client
	topicARN string
}

// New creates a new SNS Publisher.
func New(client *awssns.Client, topicARN string) *Publisher {
	return &Publisher{client: client, topicARN: topicARN}
}

// PublishHandler returns an events.Handler that publishes events to SNS.
// Register it as a subscriber on the local EventBus.
func (p *Publisher) PublishHandler() events.Handler {
	return func(ctx context.Context, evt events.Event) error {
		payload, err := json.Marshal(evt)
		if err != nil {
			log.Printf("sns: failed to marshal event: %v", err)
			return err
		}

		_, err = p.client.Publish(ctx, &awssns.PublishInput{
			TopicArn: aws.String(p.topicARN),
			Message:  aws.String(string(payload)),
			MessageAttributes: map[string]snstypes.MessageAttributeValue{
				"eventType": {
					DataType:    aws.String("String"),
					StringValue: aws.String(string(evt.Type)),
				},
			},
		})
		if err != nil {
			log.Printf("sns: failed to publish %s event: %v", evt.Type, err)
			return err
		}
		return nil
	}
}
