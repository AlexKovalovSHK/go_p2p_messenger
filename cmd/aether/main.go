package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/user/aether/internal/identity"
	"github.com/user/aether/internal/storage"
	"github.com/user/aether/internal/transport"
	"github.com/user/aether/internal/event"
	"github.com/user/aether/internal/api"
	"github.com/user/aether/internal/logic"
	"github.com/user/aether/internal/ui"
	"github.com/user/aether/internal/ui/screens"
	"github.com/user/aether/internal/ui/viewmodel"
	"github.com/libp2p/go-libp2p/core/peer"
)

func main() {
	// 0. Event Bus setup (Sprint 4)
	bus := event.NewBus()

	// 1. Storage setup
	dataDir := "data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data dir: %v", err)
	}

	dbPath := filepath.Join(dataDir, "aether.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		log.Fatalf("Failed to open storage: %v", err)
	}
	defer db.Close()

	if err := storage.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Database initialized and migrations applied.")

	msgRepo := storage.NewMessageRepository(db)

	// 2. Identity setup
	keyPath := filepath.Join(dataDir, "identity.key")
	idMgr := identity.NewIdentityManager(keyPath)

	var id *identity.Identity
	if idMgr.HasKey() {
		id, err = idMgr.Load()
		if err != nil {
			log.Fatalf("Failed to load identity: %v", err)
		}
		log.Println("Loaded existing identity from keystore.")
	} else {
		id, err = idMgr.Generate()
		if err != nil {
			log.Fatalf("Failed to generate identity: %v", err)
		}
		log.Println("Generated new identity and saved to keystore.")
	}

	processor := logic.NewMessageProcessor(id.PrivateKey, bus, msgRepo)

	// 3. Transport setup (Sprint 2)
	ctx := context.Background()
	p2pHost, err := transport.NewLibp2pHost(id, 0)
	if err != nil {
		log.Fatalf("Failed to create libp2p host: %v", err)
	}
	defer p2pHost.Close()

	tPort := transport.NewLibp2pTransport(p2pHost)
	tPort.Subscribe(func(from peer.ID, payload []byte) {
		log.Printf(">>> Received message from %s, processing...", from)
		if err := processor.ProcessIncoming(ctx, from, payload); err != nil {
			log.Printf("Error processing incoming message from %s: %v", from, err)
		}
	})

	// Start Discovery
	_, err = transport.StartMDNS(p2pHost, "aether-messenger", func(pi peer.AddrInfo) {
		log.Printf("mDNS: Found peer %s, connecting...", pi.ID)
		if err := p2pHost.Connect(ctx, pi); err != nil {
			log.Printf("mDNS: Failed to connect to %s: %v", pi.ID, err)
		} else {
			log.Printf("mDNS: Connected to %s", pi.ID)
		}
	})
	if err != nil {
		log.Printf("Failed to start mDNS: %v", err)
	}

	transport.SetupAutoNAT(p2pHost, bus, func(status transport.ReachabilityStatus) {
		log.Printf("Network status changed: %v", status)
	})

	// 4. API & UI Initialization (Sprint 4 & 5)
	chatSvc := api.NewChatService(msgRepo, processor, tPort, bus)
	nodeSvc := api.NewNodeService(id, tPort, bus)

	nav := ui.NewAppNavigator("Aether Messenger")
	mainVM := viewmodel.NewMainViewModel()
	
	// Define navigation transitions
	var showMainChat func()
	var showDirectChat func(peerID string)

	showMainChat = func() {
		listVM := viewmodel.NewChatListViewModel(chatSvc, bus)
		listVM.Refresh(ctx, chatSvc)
		listVM.Watch(ctx, chatSvc)
		
		listScreen := screens.NewChatListScreen(listVM, func(peerID string) {
			showDirectChat(peerID)
		})
		
		nav.SetMaster(listScreen.Render())
		nav.SetSplit() // Ensure we are in split mode
	}

	showDirectChat = func(peerID string) {
		mainVM.CurrentChatID.Set(peerID)
		
		chatVM := viewmodel.NewDirectChatViewModel(chatSvc, bus, peerID)
		chatVM.LoadMessages(ctx)
		chatVM.Watch(ctx)
		
		chatScreen := screens.NewDirectChatScreen(chatVM, chatSvc, func() {
			// In Master-Detail, "back" might just clear the detail
			nav.SetDetail(nil) 
		})
		nav.SetDetail(chatScreen.Render())
	}

	// Initial screen logic
	if idMgr.HasKey() {
		showMainChat()
	} else {
		idScreen := screens.NewIdentityScreen(nodeSvc, func() {
			showMainChat()
		})
		nav.SetContent(idScreen.Render())
	}

	// 5. Final startup logic
	log.Printf("Aether Node Started: %s", id.DeviceID())
	nav.ShowAndRun()
}
