package main

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/mercury/cmd/portal/lib"
	"github.com/mercury/cmd/portal/pb"
	"github.com/mercury/pkg/clients/inventory"
	"github.com/mercury/pkg/config"
	"github.com/mercury/pkg/middleware"
	"github.com/mercury/pkg/server"
	"github.com/mercury/pkg/websocketrpc"
	"github.com/sirupsen/logrus"
)

func main() {
	cfg := config.NewConfig("yaml")
	err := cfg.LoadPaths(config.DefaultConfigPaths)
	if err != nil {
		panic(err.Error())
	}
	port := cfg.SetDefaultString("web_port", "80", false)
	logLevel := cfg.SetDefaultString("log_level", "info", false)
	environment := cfg.SetDefaultString("environment", "local", false)
	statsdAddr := cfg.SetDefaultString("statsd_addr", "telegraf:8125", false)
	amqpURL := cfg.SetDefaultString("amqp_url", "amqp://guest:guest@rabbitmq:5672/", false)

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logger.SetLevel(level)

	statsdClient := middleware.NewStatsdClient(statsdAddr, "portal")

	inventoryClient, err := inventory.NewClient(amqpURL)
	if err != nil {
		logrus.Fatal(err)
	}
	defer inventoryClient.Close()

	h := lib.NewPortalHandlers(inventoryClient)

	rpc := websocketrpc.NewWsRpcHandler(websocketrpc.WsRpcOpt{
		Name: "portal",
	})
	rpc.Register(uint16(pb.GameMsg_TRADE_REQUEST), websocketrpc.ProtoHandler(
		func() *pb.TradeRequest { return &pb.TradeRequest{} },
		h.HandleTradeRequest,
	))
	rpc.Register(uint16(pb.GameMsg_INVENTORY_GET_REQUEST), websocketrpc.ProtoHandler(
		func() *pb.InventoryGetRequest { return &pb.InventoryGetRequest{} },
		h.HandleInventoryGetRequest,
	))
	rpc.Register(uint16(pb.GameMsg_INVENTORY_ADD_ITEM), websocketrpc.ProtoHandler(
		func() *pb.InventoryAddItemRequest { return &pb.InventoryAddItemRequest{} },
		h.HandleInventoryAddItem,
	))
	rpc.Register(uint16(pb.GameMsg_INVENTORY_ADD_ITEM_TO_SLOT), websocketrpc.ProtoHandler(
		func() *pb.InventoryAddItemToSlotRequest { return &pb.InventoryAddItemToSlotRequest{} },
		h.HandleInventoryAddItemToSlot,
	))

	e := echo.New()
	v1 := e.Group("api/v1",
		middleware.UseLogger(logger, environment),
		middleware.UseStatsd(statsdClient))
	gsv1 := v1.Group("/portal")
	gsv1.GET("/ws", rpc.Handle)
	if err := server.Serve(e, fmt.Sprintf(":%s", port)); err != nil {
		logger.Fatal(err)
	}
}
