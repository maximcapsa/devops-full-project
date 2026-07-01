// Package events holds the Kafka topic names shared by producers and consumers.
// Event payloads themselves are the generated types in gen/events/v1,
// serialized as protojson on the wire.
package events

const (
	TopicOrdersPlaced      = "orders.placed"
	TopicPaymentsCompleted = "payments.completed"
	TopicPaymentsFailed    = "payments.failed"
	TopicInventoryReserved = "inventory.reserved"
	TopicInventoryRejected = "inventory.rejected"
)
