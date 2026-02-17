//go:build integration

package dynamo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"github.com/jeffwnelson/sodatracker/internal/domain"
)

const testTableName = "SodaTracker-test"

func setupTestStore(t *testing.T) *DynamoStore {
	t.Helper()

	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8000"
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("us-west-2"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "test")),
	)
	if err != nil {
		t.Fatalf("failed to load AWS config: %v", err)
	}

	client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	// Create table (ignore if exists)
	_, _ = client.CreateTable(context.Background(), &dynamodb.CreateTableInput{
		TableName: aws.String(testTableName),
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("PK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("SK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("GSI1PK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("GSI1SK"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("PK"), KeyType: types.KeyTypeHash},
			{AttributeName: aws.String("SK"), KeyType: types.KeyTypeRange},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: aws.String("GSI1"),
				KeySchema: []types.KeySchemaElement{
					{AttributeName: aws.String("GSI1PK"), KeyType: types.KeyTypeHash},
					{AttributeName: aws.String("GSI1SK"), KeyType: types.KeyTypeRange},
				},
				Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})

	// Wait for table to become active
	for i := 0; i < 30; i++ {
		desc, err := client.DescribeTable(context.Background(), &dynamodb.DescribeTableInput{
			TableName: aws.String(testTableName),
		})
		if err == nil && desc.Table.TableStatus == types.TableStatusActive {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	store := New(client, testTableName)
	t.Cleanup(func() {
		// Clean up: delete all items (scan + batch delete)
		out, _ := client.Scan(context.Background(), &dynamodb.ScanInput{
			TableName:            aws.String(testTableName),
			ProjectionExpression: aws.String("PK, SK"),
		})
		for _, item := range out.Items {
			client.DeleteItem(context.Background(), &dynamodb.DeleteItemInput{
				TableName: aws.String(testTableName),
				Key:       map[string]types.AttributeValue{"PK": item["PK"], "SK": item["SK"]},
			})
		}
	})

	return store
}

func TestProductCRUD(t *testing.T) {
	st := setupTestStore(t)
	ctx := context.Background()

	p := &domain.Product{
		ID:         uuid.NewString(),
		UPC:        "012345678901",
		Name:       "Test Cola",
		CostPerCan: 0.35,
		ImageURL:   "https://example.com/cola.png",
	}

	if err := st.CreateProduct(ctx, p); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	got, err := st.GetProduct(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	if got == nil || got.Name != "Test Cola" {
		t.Fatalf("expected Test Cola, got %+v", got)
	}

	gotByUPC, err := st.GetProductByUPC(ctx, "012345678901")
	if err != nil {
		t.Fatalf("GetProductByUPC: %v", err)
	}
	if gotByUPC == nil || gotByUPC.ID != p.ID {
		t.Fatalf("expected product by UPC, got %+v", gotByUPC)
	}

	products, err := st.ListProducts(ctx)
	if err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	if len(products) < 1 {
		t.Fatal("expected at least 1 product")
	}
}

func TestMachineSlotsAndStatus(t *testing.T) {
	st := setupTestStore(t)
	ctx := context.Background()

	m := &domain.Machine{
		ID:        uuid.NewString(),
		Name:      "Test Machine",
		Location:  "Office",
		SlotCount: 6,
	}
	if err := st.CreateMachine(ctx, m); err != nil {
		t.Fatalf("CreateMachine: %v", err)
	}

	got, err := st.GetMachine(ctx, m.ID)
	if err != nil {
		t.Fatalf("GetMachine: %v", err)
	}
	if got == nil || got.Name != "Test Machine" {
		t.Fatalf("expected Test Machine, got %+v", got)
	}

	machines, err := st.ListMachines(ctx)
	if err != nil {
		t.Fatalf("ListMachines: %v", err)
	}
	if len(machines) < 1 {
		t.Fatal("expected at least 1 machine")
	}

	slots := []domain.Slot{
		{Position: 1, ProductID: "prod1", Capacity: 10, LowThreshold: 2},
		{Position: 2, ProductID: "prod2", Capacity: 10, LowThreshold: 2},
	}
	if err := st.SetSlots(ctx, m.ID, slots); err != nil {
		t.Fatalf("SetSlots: %v", err)
	}

	gotSlots, err := st.GetSlots(ctx, m.ID)
	if err != nil {
		t.Fatalf("GetSlots: %v", err)
	}
	if len(gotSlots) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(gotSlots))
	}
}

func TestDispenseAndRestock(t *testing.T) {
	st := setupTestStore(t)
	ctx := context.Background()

	machineID := uuid.NewString()
	now := time.Now().UTC()

	// Create restock
	r := &domain.Restock{
		ID:        uuid.NewString(),
		MachineID: machineID,
		UserID:    "user1",
		Slots:     []domain.RestockSlot{{Position: 1, Quantity: 10}},
		Timestamp: now.Add(-1 * time.Hour),
	}
	if err := st.CreateRestock(ctx, r); err != nil {
		t.Fatalf("CreateRestock: %v", err)
	}

	// Create dispense
	d := &domain.Dispense{
		ID:           uuid.NewString(),
		MachineID:    machineID,
		ProductID:    "prod1",
		SlotPosition: 1,
		SourceType:   "gpio",
		Timestamp:    now,
	}
	if err := st.CreateDispense(ctx, d); err != nil {
		t.Fatalf("CreateDispense: %v", err)
	}

	// List dispenses
	dispenses, err := st.ListDispenses(ctx, machineID, now.Add(-2*time.Hour), 100)
	if err != nil {
		t.Fatalf("ListDispenses: %v", err)
	}
	if len(dispenses) != 1 {
		t.Fatalf("expected 1 dispense, got %d", len(dispenses))
	}

	// Count dispenses since restock
	count, err := st.CountDispensesSince(ctx, machineID, 1, r.Timestamp)
	if err != nil {
		t.Fatalf("CountDispensesSince: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	// Get last restock
	lastRestock, restockSlot, err := st.GetLastRestock(ctx, machineID, 1)
	if err != nil {
		t.Fatalf("GetLastRestock: %v", err)
	}
	if lastRestock == nil || restockSlot == nil {
		t.Fatal("expected restock")
	}
	if restockSlot.Quantity != 10 {
		t.Fatalf("expected quantity 10, got %d", restockSlot.Quantity)
	}

	// List restocks
	restocks, err := st.ListRestocks(ctx, machineID, 100)
	if err != nil {
		t.Fatalf("ListRestocks: %v", err)
	}
	if len(restocks) != 1 {
		t.Fatalf("expected 1 restock, got %d", len(restocks))
	}
}

func TestUserCRUD(t *testing.T) {
	st := setupTestStore(t)
	ctx := context.Background()

	u := &domain.User{
		ID:           uuid.NewString(),
		Username:     "testadmin",
		PasswordHash: "$2a$10$fakehash",
		Role:         "admin",
	}
	if err := st.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := st.GetUserByUsername(ctx, "testadmin")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if got == nil || got.Username != "testadmin" {
		t.Fatalf("expected testadmin, got %+v", got)
	}

	// Non-existent user
	missing, err := st.GetUserByUsername(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetUserByUsername (missing): %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil, got %+v", missing)
	}
}

func TestAlertConfigCRUD(t *testing.T) {
	st := setupTestStore(t)
	ctx := context.Background()

	machineID := uuid.NewString()
	ac := &domain.AlertConfig{
		ID:           uuid.NewString(),
		MachineID:    machineID,
		SlotPosition: 1,
		LowThreshold: 3,
		Enabled:      true,
	}
	if err := st.CreateAlertConfig(ctx, ac); err != nil {
		t.Fatalf("CreateAlertConfig: %v", err)
	}

	configs, err := st.ListAlertConfigs(ctx, machineID)
	if err != nil {
		t.Fatalf("ListAlertConfigs: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].LowThreshold != 3 {
		t.Fatalf("expected threshold 3, got %d", configs[0].LowThreshold)
	}

	ac.LowThreshold = 5
	if err := st.UpdateAlertConfig(ctx, ac); err != nil {
		t.Fatalf("UpdateAlertConfig: %v", err)
	}

	configs, _ = st.ListAlertConfigs(ctx, machineID)
	if configs[0].LowThreshold != 5 {
		t.Fatalf("expected threshold 5 after update, got %d", configs[0].LowThreshold)
	}

	if err := st.DeleteAlertConfig(ctx, ac.ID); err != nil {
		t.Fatalf("DeleteAlertConfig: %v", err)
	}

	configs, _ = st.ListAlertConfigs(ctx, machineID)
	if len(configs) != 0 {
		t.Fatalf("expected 0 configs after delete, got %d", len(configs))
	}
}

func TestAlertEventCRUD(t *testing.T) {
	st := setupTestStore(t)
	ctx := context.Background()

	machineID := uuid.NewString()
	ae := &domain.AlertEvent{
		ID:           uuid.NewString(),
		MachineID:    machineID,
		SlotPosition: 1,
		ProductID:    "prod1",
		AlertType:    "low_stock",
		CurrentStock: 2,
		Threshold:    3,
		Cleared:      false,
		Timestamp:    time.Now().UTC(),
	}
	if err := st.CreateAlertEvent(ctx, ae); err != nil {
		t.Fatalf("CreateAlertEvent: %v", err)
	}

	// Get active alert
	active, err := st.GetActiveAlert(ctx, machineID, 1, "low_stock")
	if err != nil {
		t.Fatalf("GetActiveAlert: %v", err)
	}
	if active == nil {
		t.Fatal("expected active alert")
	}

	// List active events
	events, err := st.ListAlertEvents(ctx, machineID, true, 100)
	if err != nil {
		t.Fatalf("ListAlertEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// Clear events
	if err := st.ClearAlertEvents(ctx, machineID, 1); err != nil {
		t.Fatalf("ClearAlertEvents: %v", err)
	}

	// Verify cleared
	active, _ = st.GetActiveAlert(ctx, machineID, 1, "low_stock")
	if active != nil {
		t.Fatal("expected no active alert after clearing")
	}

	// All events (including cleared) should still exist
	events, _ = st.ListAlertEvents(ctx, machineID, false, 100)
	if len(events) != 1 {
		t.Fatalf("expected 1 event total, got %d", len(events))
	}
	if !events[0].Cleared {
		t.Fatal("expected event to be cleared")
	}
}
