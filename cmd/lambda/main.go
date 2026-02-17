package main

import (
	"context"
	"log"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-lambda-go/lambda"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"

	"github.com/jeffwnelson/sodatracker/internal/api"
	"github.com/jeffwnelson/sodatracker/internal/config"
	"github.com/jeffwnelson/sodatracker/internal/events"
	"github.com/jeffwnelson/sodatracker/internal/events/local"
	"github.com/jeffwnelson/sodatracker/internal/events/sns"
	"github.com/jeffwnelson/sodatracker/internal/seed"
	"github.com/jeffwnelson/sodatracker/internal/service"
	"github.com/jeffwnelson/sodatracker/internal/store/dynamo"
)

func main() {
	cfg := config.Load()

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	// DynamoDB store
	ddbClient := dynamodb.NewFromConfig(awsCfg)
	st := dynamo.New(ddbClient, cfg.TableName)

	// Seed admin user on cold start
	seed.AdminUser(st, cfg.AdminPassword)

	// Local event bus (synchronous, in-process — same as self-hosted)
	bus := local.New()

	svc := service.New(st, cfg.JWTSecret, bus)

	// Wire: dispense event → alert evaluation
	bus.Subscribe(events.EventDispense, func(ctx context.Context, evt events.Event) error {
		machineID, _ := evt.Data["machineId"].(string)
		slotPos := 0
		switch v := evt.Data["slotPosition"].(type) {
		case int:
			slotPos = v
		case float64:
			slotPos = int(v)
		}
		if machineID != "" && slotPos > 0 {
			return svc.EvaluateStockAlert(ctx, machineID, slotPos)
		}
		return nil
	})

	// Optionally wire SNS publisher for external fan-out
	if cfg.SNSTopicARN != "" {
		snsClient := awssns.NewFromConfig(awsCfg)
		publisher := sns.New(snsClient, cfg.SNSTopicARN)
		bus.Subscribe(events.EventDispense, publisher.PublishHandler())
		bus.Subscribe(events.EventLowStock, publisher.PublishHandler())
		bus.Subscribe(events.EventOutOfStock, publisher.PublishHandler())
		bus.Subscribe(events.EventStockCleared, publisher.PublishHandler())
	}

	router := api.NewRouter(st, svc, cfg.APIKey, cfg.JWTSecret, "")
	adapter := chiadapter.NewV2(router)
	lambda.Start(adapter.ProxyWithContextV2)
}
