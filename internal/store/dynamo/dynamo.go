package dynamo

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/jeffwnelson/sodatracker/internal/domain"
)

const gsi1Name = "GSI1"

// DynamoStore implements store.Store backed by DynamoDB single-table design.
type DynamoStore struct {
	client    *dynamodb.Client
	tableName string
}

// New creates a new DynamoStore.
func New(client *dynamodb.Client, tableName string) *DynamoStore {
	return &DynamoStore{client: client, tableName: tableName}
}

func (d *DynamoStore) Close() error { return nil }

// --- Products ---

func (d *DynamoStore) CreateProduct(ctx context.Context, p *domain.Product) error {
	item := marshalProduct(p)
	_, err := d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &d.tableName,
		Item:      item,
	})
	return err
}

func (d *DynamoStore) GetProduct(ctx context.Context, id string) (*domain.Product, error) {
	out, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &d.tableName,
		Key: map[string]types.AttributeValue{
			"PK": s(productPK(id)),
			"SK": s(productSK(id)),
		},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, nil
	}
	p := unmarshalProduct(out.Item)
	return &p, nil
}

func (d *DynamoStore) GetProductByUPC(ctx context.Context, upc string) (*domain.Product, error) {
	out, err := d.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              &d.tableName,
		IndexName:              aws.String(gsi1Name),
		KeyConditionExpression: aws.String("GSI1PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": s(productGSI1PK(upc)),
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, err
	}
	if len(out.Items) == 0 {
		return nil, nil
	}
	p := unmarshalProduct(out.Items[0])
	return &p, nil
}

func (d *DynamoStore) ListProducts(ctx context.Context) ([]domain.Product, error) {
	out, err := d.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        &d.tableName,
		FilterExpression: aws.String("#t = :type"),
		ExpressionAttributeNames: map[string]string{
			"#t": "_type",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":type": s("Product"),
		},
	})
	if err != nil {
		return nil, err
	}
	products := make([]domain.Product, 0, len(out.Items))
	for _, item := range out.Items {
		products = append(products, unmarshalProduct(item))
	}
	sort.Slice(products, func(i, j int) bool {
		return products[i].Name < products[j].Name
	})
	return products, nil
}

func (d *DynamoStore) DeleteProduct(ctx context.Context, id string) error {
	_, err := d.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &d.tableName,
		Key: map[string]types.AttributeValue{
			"PK": s(productPK(id)),
			"SK": s(productSK(id)),
		},
	})
	return err
}

// --- Machines ---

func (d *DynamoStore) CreateMachine(ctx context.Context, m *domain.Machine) error {
	item := marshalMachine(m)
	_, err := d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &d.tableName,
		Item:      item,
	})
	return err
}

func (d *DynamoStore) GetMachine(ctx context.Context, id string) (*domain.Machine, error) {
	out, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &d.tableName,
		Key: map[string]types.AttributeValue{
			"PK": s(machinePK(id)),
			"SK": s(machineSK(id)),
		},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, nil
	}
	m := unmarshalMachine(out.Item)
	return &m, nil
}

func (d *DynamoStore) ListMachines(ctx context.Context) ([]domain.Machine, error) {
	out, err := d.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              &d.tableName,
		IndexName:              aws.String(gsi1Name),
		KeyConditionExpression: aws.String("GSI1PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": s(machineGSI1PK()),
		},
	})
	if err != nil {
		return nil, err
	}
	machines := make([]domain.Machine, 0, len(out.Items))
	for _, item := range out.Items {
		machines = append(machines, unmarshalMachine(item))
	}
	return machines, nil
}

func (d *DynamoStore) DeleteMachine(ctx context.Context, id string) error {
	// Query all items under MACHINE#{id} PK and delete them in batches
	pk := machinePK(id)
	var lastKey map[string]types.AttributeValue
	for {
		input := &dynamodb.QueryInput{
			TableName:              &d.tableName,
			KeyConditionExpression: aws.String("PK = :pk"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk": s(pk),
			},
			ProjectionExpression: aws.String("PK, SK"),
		}
		if lastKey != nil {
			input.ExclusiveStartKey = lastKey
		}
		out, err := d.client.Query(ctx, input)
		if err != nil {
			return err
		}

		// Batch delete in groups of 25
		for i := 0; i < len(out.Items); i += 25 {
			end := i + 25
			if end > len(out.Items) {
				end = len(out.Items)
			}
			var requests []types.WriteRequest
			for _, item := range out.Items[i:end] {
				requests = append(requests, types.WriteRequest{
					DeleteRequest: &types.DeleteRequest{
						Key: map[string]types.AttributeValue{
							"PK": item["PK"],
							"SK": item["SK"],
						},
					},
				})
			}
			_, err := d.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]types.WriteRequest{
					d.tableName: requests,
				},
			})
			if err != nil {
				return err
			}
		}

		lastKey = out.LastEvaluatedKey
		if lastKey == nil {
			break
		}
	}
	return nil
}

// --- Slots ---

func (d *DynamoStore) SetSlots(ctx context.Context, machineID string, slots []domain.Slot) error {
	// First, delete existing slots
	existing, err := d.GetSlots(ctx, machineID)
	if err != nil {
		return err
	}
	for _, sl := range existing {
		_, err := d.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: &d.tableName,
			Key: map[string]types.AttributeValue{
				"PK": s(slotPK(machineID)),
				"SK": s(slotSK(sl.Position)),
			},
		})
		if err != nil {
			return err
		}
	}

	// Write new slots using BatchWriteItem (max 25 per batch)
	if len(slots) == 0 {
		return nil
	}

	var requests []types.WriteRequest
	for i := range slots {
		requests = append(requests, types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: marshalSlot(machineID, &slots[i]),
			},
		})
	}

	for i := 0; i < len(requests); i += 25 {
		end := i + 25
		if end > len(requests) {
			end = len(requests)
		}
		_, err := d.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				d.tableName: requests[i:end],
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *DynamoStore) GetSlots(ctx context.Context, machineID string) ([]domain.Slot, error) {
	out, err := d.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              &d.tableName,
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     s(slotPK(machineID)),
			":prefix": s("SLOT#"),
		},
	})
	if err != nil {
		return nil, err
	}
	slots := make([]domain.Slot, 0, len(out.Items))
	for _, item := range out.Items {
		slots = append(slots, unmarshalSlot(item))
	}
	return slots, nil
}

// --- Dispenses ---

func (d *DynamoStore) CreateDispense(ctx context.Context, disp *domain.Dispense) error {
	item := marshalDispense(disp)
	_, err := d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &d.tableName,
		Item:      item,
	})
	return err
}

func (d *DynamoStore) ListDispenses(ctx context.Context, machineID string, since time.Time, limit int) ([]domain.Dispense, error) {
	sinceStr := since.UTC().Format(time.RFC3339Nano)
	out, err := d.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              &d.tableName,
		KeyConditionExpression: aws.String("PK = :pk AND SK BETWEEN :start AND :end"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":    s(dispensePK(machineID)),
			":start": s(fmt.Sprintf("DISPENSE#%s", sinceStr)),
			":end":   s("DISPENSE#~"), // ~ sorts after all RFC3339 timestamps
		},
		ScanIndexForward: aws.Bool(false), // descending
		Limit:            aws.Int32(int32(limit)),
	})
	if err != nil {
		return nil, err
	}
	dispenses := make([]domain.Dispense, 0, len(out.Items))
	for _, item := range out.Items {
		dispenses = append(dispenses, unmarshalDispense(item))
	}
	return dispenses, nil
}

func (d *DynamoStore) CountDispensesSince(ctx context.Context, machineID string, slotPosition int, since time.Time) (int, error) {
	sinceStr := since.UTC().Format(time.RFC3339Nano)
	out, err := d.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              &d.tableName,
		IndexName:              aws.String(gsi1Name),
		KeyConditionExpression: aws.String("GSI1PK = :pk AND GSI1SK >= :start"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":    s(dispenseGSI1PK(machineID, slotPosition)),
			":start": s(fmt.Sprintf("DISPENSE#%s", sinceStr)),
		},
		Select: types.SelectCount,
	})
	if err != nil {
		return 0, err
	}
	return int(out.Count), nil
}

// --- Restocks ---

func (d *DynamoStore) CreateRestock(ctx context.Context, r *domain.Restock) error {
	item := marshalRestock(r)
	_, err := d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &d.tableName,
		Item:      item,
	})
	return err
}

func (d *DynamoStore) GetLastRestock(ctx context.Context, machineID string, slotPosition int) (*domain.Restock, *domain.RestockSlot, error) {
	// Query restocks in reverse chronological order, walk until we find one containing the target slot
	out, err := d.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              &d.tableName,
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     s(restockPK(machineID)),
			":prefix": s("RESTOCK#"),
		},
		ScanIndexForward: aws.Bool(false), // descending
	})
	if err != nil {
		return nil, nil, err
	}

	for _, item := range out.Items {
		r, err := unmarshalRestock(item)
		if err != nil {
			return nil, nil, err
		}
		for i := range r.Slots {
			if r.Slots[i].Position == slotPosition {
				return &r, &r.Slots[i], nil
			}
		}
	}
	return nil, nil, nil
}

func (d *DynamoStore) ListRestocks(ctx context.Context, machineID string, limit int) ([]domain.Restock, error) {
	out, err := d.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              &d.tableName,
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     s(restockPK(machineID)),
			":prefix": s("RESTOCK#"),
		},
		ScanIndexForward: aws.Bool(false),
		Limit:            aws.Int32(int32(limit)),
	})
	if err != nil {
		return nil, err
	}
	restocks := make([]domain.Restock, 0, len(out.Items))
	for _, item := range out.Items {
		r, err := unmarshalRestock(item)
		if err != nil {
			return nil, err
		}
		restocks = append(restocks, r)
	}
	return restocks, nil
}

// --- Users ---

func (d *DynamoStore) CreateUser(ctx context.Context, u *domain.User) error {
	item := marshalUser(u)
	_, err := d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &d.tableName,
		Item:      item,
	})
	return err
}

func (d *DynamoStore) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	out, err := d.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              &d.tableName,
		IndexName:              aws.String(gsi1Name),
		KeyConditionExpression: aws.String("GSI1PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": s(userGSI1PK(username)),
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, err
	}
	if len(out.Items) == 0 {
		return nil, nil
	}
	u := unmarshalUser(out.Items[0])
	return &u, nil
}

func (d *DynamoStore) UpdateUserTheme(ctx context.Context, userID string, theme string) error {
	_, err := d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &d.tableName,
		Key: map[string]types.AttributeValue{
			"PK": s(userPK(userID)),
			"SK": s(userSK(userID)),
		},
		UpdateExpression: aws.String("SET theme = :t"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":t": s(theme),
		},
	})
	return err
}

// --- Admin ---

func (d *DynamoStore) ResetAllData(ctx context.Context) error {
	var lastKey map[string]types.AttributeValue
	for {
		input := &dynamodb.ScanInput{
			TableName:            &d.tableName,
			ProjectionExpression: aws.String("PK, SK"),
		}
		if lastKey != nil {
			input.ExclusiveStartKey = lastKey
		}
		out, err := d.client.Scan(ctx, input)
		if err != nil {
			return err
		}
		for i := 0; i < len(out.Items); i += 25 {
			end := i + 25
			if end > len(out.Items) {
				end = len(out.Items)
			}
			var requests []types.WriteRequest
			for _, item := range out.Items[i:end] {
				requests = append(requests, types.WriteRequest{
					DeleteRequest: &types.DeleteRequest{
						Key: map[string]types.AttributeValue{
							"PK": item["PK"],
							"SK": item["SK"],
						},
					},
				})
			}
			_, err := d.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]types.WriteRequest{
					d.tableName: requests,
				},
			})
			if err != nil {
				return err
			}
		}
		lastKey = out.LastEvaluatedKey
		if lastKey == nil {
			break
		}
	}
	return nil
}

// --- Alert Configs ---

func (d *DynamoStore) CreateAlertConfig(ctx context.Context, ac *domain.AlertConfig) error {
	// Upsert: PK + SK is unique per machine+slotPosition
	item := marshalAlertConfig(ac)
	_, err := d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &d.tableName,
		Item:      item,
	})
	return err
}

func (d *DynamoStore) ListAlertConfigs(ctx context.Context, machineID string) ([]domain.AlertConfig, error) {
	out, err := d.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              &d.tableName,
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     s(alertConfigPK(machineID)),
			":prefix": s("ALERTCFG#"),
		},
	})
	if err != nil {
		return nil, err
	}
	configs := make([]domain.AlertConfig, 0, len(out.Items))
	for _, item := range out.Items {
		configs = append(configs, unmarshalAlertConfig(item))
	}
	return configs, nil
}

func (d *DynamoStore) UpdateAlertConfig(ctx context.Context, ac *domain.AlertConfig) error {
	// We need PK+SK to update. AlertConfig SK is derived from machineId+slotPosition.
	// The caller passes the full AlertConfig with the ID. We need to find the item first
	// to get its PK/SK, or we can reconstruct them if we have machineId and slotPosition.
	_, err := d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &d.tableName,
		Key: map[string]types.AttributeValue{
			"PK": s(alertConfigPK(ac.MachineID)),
			"SK": s(alertConfigSK(ac.SlotPosition)),
		},
		UpdateExpression: aws.String("SET lowThreshold = :lt, enabled = :en"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":lt": n(ac.LowThreshold),
			":en": b(ac.Enabled),
		},
	})
	return err
}

func (d *DynamoStore) DeleteAlertConfig(ctx context.Context, id string) error {
	// We only have the ID, need to scan to find the item
	out, err := d.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        &d.tableName,
		FilterExpression: aws.String("#t = :type AND id = :id"),
		ExpressionAttributeNames: map[string]string{
			"#t": "_type",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":type": s("AlertConfig"),
			":id":   s(id),
		},
	})
	if err != nil {
		return err
	}
	if len(out.Items) == 0 {
		return nil
	}

	_, err = d.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &d.tableName,
		Key: map[string]types.AttributeValue{
			"PK": out.Items[0]["PK"],
			"SK": out.Items[0]["SK"],
		},
	})
	return err
}

// --- Alert Events ---

func (d *DynamoStore) CreateAlertEvent(ctx context.Context, ae *domain.AlertEvent) error {
	item := marshalAlertEvent(ae)
	_, err := d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &d.tableName,
		Item:      item,
	})
	return err
}

func (d *DynamoStore) ListAlertEvents(ctx context.Context, machineID string, activeOnly bool, limit int) ([]domain.AlertEvent, error) {
	input := &dynamodb.QueryInput{
		TableName:              &d.tableName,
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     s(alertEventPK(machineID)),
			":prefix": s("ALERTEVT#"),
		},
		ScanIndexForward: aws.Bool(false),
		Limit:            aws.Int32(int32(limit)),
	}

	if activeOnly {
		input.FilterExpression = aws.String("cleared = :false")
		input.ExpressionAttributeValues[":false"] = b(false)
	}

	out, err := d.client.Query(ctx, input)
	if err != nil {
		return nil, err
	}
	events := make([]domain.AlertEvent, 0, len(out.Items))
	for _, item := range out.Items {
		events = append(events, unmarshalAlertEvent(item))
	}
	return events, nil
}

func (d *DynamoStore) ClearAlertEvents(ctx context.Context, machineID string, slotPosition int) error {
	// Find active alerts for this machine+slot using GSI1
	out, err := d.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              &d.tableName,
		IndexName:              aws.String(gsi1Name),
		KeyConditionExpression: aws.String("GSI1PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": s(activeAlertGSI1PK(machineID, slotPosition)),
		},
	})
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, item := range out.Items {
		pk := item["PK"]
		sk := item["SK"]
		id := getS(item, "id")
		alertType := getS(item, "alertType")

		_, err := d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: &d.tableName,
			Key: map[string]types.AttributeValue{
				"PK": pk,
				"SK": sk,
			},
			UpdateExpression: aws.String("SET cleared = :true, clearedAt = :now, GSI1PK = :newGSI1PK, GSI1SK = :newGSI1SK"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":true":      b(true),
				":now":       s(now),
				":newGSI1PK": s(clearedAlertGSI1PK(machineID, slotPosition)),
				":newGSI1SK": s(alertEventGSI1SK(alertType, id)),
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *DynamoStore) GetActiveAlert(ctx context.Context, machineID string, slotPosition int, alertType string) (*domain.AlertEvent, error) {
	out, err := d.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              &d.tableName,
		IndexName:              aws.String(gsi1Name),
		KeyConditionExpression: aws.String("GSI1PK = :pk AND begins_with(GSI1SK, :prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     s(activeAlertGSI1PK(machineID, slotPosition)),
			":prefix": s(alertType + "#"),
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, err
	}
	if len(out.Items) == 0 {
		return nil, nil
	}
	ae := unmarshalAlertEvent(out.Items[0])
	return &ae, nil
}
