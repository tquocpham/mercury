package lib

import (
	"context"
	"fmt"

	"github.com/mercury/cmd/portal/pb"
	"github.com/mercury/pkg/clients/inventory"
)

type PortalHandlers interface {
	HandleTradeRequest(ctx context.Context, req *pb.TradeRequest) (*pb.TradeResponse, error)
	HandleInventoryGetRequest(ctx context.Context, req *pb.InventoryGetRequest) (*pb.InventoryGetResponse, error)
	HandleInventoryAddItem(ctx context.Context, req *pb.InventoryAddItemRequest) (*pb.InventoryGetResponse, error)
	HandleInventoryAddItemToSlot(ctx context.Context, req *pb.InventoryAddItemToSlotRequest) (*pb.InventoryGetResponse, error)
}

type portalHandlers struct {
	inventoryClient inventory.RMQClient
}

func NewPortalHandlers(inventoryClient inventory.RMQClient) PortalHandlers {
	return &portalHandlers{
		inventoryClient: inventoryClient,
	}
}

func (h *portalHandlers) HandleTradeRequest(ctx context.Context, req *pb.TradeRequest) (*pb.TradeResponse, error) {
	// 1. Do your database/microservice logic here
	fmt.Printf("Player is trading item: %d\n", req.ItemId)

	// 2. Return the Protobuf response
	return &pb.TradeResponse{
		Success: true,
		Message: "Trade completed successfully",
	}, nil
}

func (h *portalHandlers) HandleInventoryAddItem(ctx context.Context, req *pb.InventoryAddItemRequest) (*pb.InventoryGetResponse, error) {
	response, err := h.inventoryClient.AddItem(ctx, req.PlayerId, req.ItemId, req.OrderId, int(req.Amount), int(req.MaxStack))
	if err != nil {
		return nil, inventory.ConvertRPCError(err)
	}
	items := make([]*pb.Item, 0, len(response.Inventory))
	for _, slot := range response.Inventory {
		if slot.ItemID != "" {
			items = append(items, &pb.Item{
				ItemId: slot.ItemID,
				Amount: int32(slot.Amount),
			})
		}
	}
	return &pb.InventoryGetResponse{
		PlayerId:  "123",
		Inventory: items,
	}, nil
}

func (h *portalHandlers) HandleInventoryAddItemToSlot(ctx context.Context, req *pb.InventoryAddItemToSlotRequest) (*pb.InventoryGetResponse, error) {
	response, err := h.inventoryClient.AddItemToSlot(ctx, req.PlayerId, req.ItemId, req.OrderId, int(req.SlotId), int(req.Amount), int(req.MaxStack))
	if err != nil {
		return nil, inventory.ConvertRPCError(err)
	}
	items := make([]*pb.Item, 0, len(response.Inventory))
	for _, slot := range response.Inventory {
		if slot.ItemID != "" {
			items = append(items, &pb.Item{
				ItemId: slot.ItemID,
				Amount: int32(slot.Amount),
			})
		}
	}
	return &pb.InventoryGetResponse{
		PlayerId:  "123",
		Inventory: items,
	}, nil
}
func (h *portalHandlers) HandleInventoryGetRequest(ctx context.Context, req *pb.InventoryGetRequest) (*pb.InventoryGetResponse, error) {
	response, err := h.inventoryClient.GetInventory(ctx, req.PlayerId)
	if err != nil {
		return nil, inventory.ConvertRPCError(err)
	}

	items := make([]*pb.Item, 0, len(response.Inventory))
	for _, slot := range response.Inventory {
		if slot.ItemID != "" {
			items = append(items, &pb.Item{
				ItemId: slot.ItemID,
				Amount: int32(slot.Amount),
			})
		}
	}

	return &pb.InventoryGetResponse{
		PlayerId:  "123",
		Inventory: items,
	}, nil
}
