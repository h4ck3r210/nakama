package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/heroiclabs/nakama-common/runtime"

	_ "google.golang.org/protobuf/proto"
)

var (
	// This is a list of functions that are called to initialize the runtime module from runtime_go.go
	EvrRuntimeModuleFns = []func(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, initializer runtime.Initializer) error{
		InitializeEvrRuntimeModule,
		EchoTaxiRuntimeModule,
	}
)

const (
	GroupGlobalDevelopers = "Global Developers"
	GroupGlobalModerators = "Global Moderators"
	GroupGlobalTesters    = "Global Testers"
)

func InitializeEvrRuntimeModule(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, initializer runtime.Initializer) (err error) {

	// Register RPC's for device linking
	rpcs := map[string]func(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error){
		"link/device":          LinkDeviceRpc,
		"link/usernamedevice":  LinkUserIdDeviceRpc,
		"signin/discord":       DiscordSignInRpc,
		"match":                MatchRpc,
		"match/prepare":        PrepareMatchRPC,
		"link":                 LinkingAppRpc,
		"evr/servicestatus":    ServiceStatusRpc,
		"importloadouts":       ImportLoadoutsRpc,
		"terminateMatch":       terminateMatchRpc,
		"matchmaker":           matchmakingStatusRpc,
		"setmatchamakerstatus": setMatchmakingStatusRpc,
	}

	for name, rpc := range rpcs {
		if err = initializer.RegisterRpc(name, rpc); err != nil {
			return fmt.Errorf("unable to register %s: %v", name, err)
		}
	}

	RegisterIndexes(initializer)

	// Create the core groups
	if err := createCoreGroups(ctx, logger, db, nk, initializer); err != nil {
		return fmt.Errorf("unable to create core groups: %v", err)
	}

	// Register the "matchmaking" handler
	if err := initializer.RegisterMatch(EvrMatchmakerModule, func(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule) (runtime.Match, error) {
		return &EvrMatch{}, nil
	}); err != nil {
		return err
	}

	// Register RPC for /api service.
	if err := initializer.RegisterRpc("evr/api", EvrApiHttpHandler); err != nil {
		return fmt.Errorf("unable to register /evr/api service: %v", err)
	}

	logger.Info("Initialized runtime module.")
	return nil
}

func createCoreGroups(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, initializer runtime.Initializer) error {
	// Create user for use by the discord bot (and core group ownership)
	userId, _, _, err := nk.AuthenticateDevice(ctx, SystemUserID, "discordbot", true)
	if err != nil {
		logger.WithField("err", err).Error("Error creating discordbot user: %v", err)
	}

	coreGroups := []string{
		GroupGlobalDevelopers,
		GroupGlobalModerators,
		GroupGlobalTesters,
	}

	for _, name := range coreGroups {
		// Search for group first
		groups, _, err := nk.GroupsList(ctx, name, "", nil, nil, 1, "")
		if err != nil {
			logger.WithField("err", err).Error("Group list error: %v", err)
		}
		if len(groups) == 0 {
			// Create a nakama group for developers
			_, err = nk.GroupCreate(ctx, userId, name, userId, "en", name, "", false, map[string]interface{}{}, 1000)
			if err != nil {
				logger.WithField("err", err).Error("Group create error: %v", err)
			}
		}
	}

	// Create a VRML group for each season
	vrmlgroups := []string{
		"VRML Season Preseason",
		"VRML Season 1",
		"VRML Season 1 Finalist",
		"VRML Season 1 Champion",
		"VRML Season 2",
		"VRML Season 2 Finalist",
		"VRML Season 2 Champion",
		"VRML Season 3",
		"VRML Season 3 Finalist",
		"VRML Season 3 Champion",
		"VRML Season 4",
		"VRML Season 4 Finalist",
		"VRML Season 4 Champion",
		"VRML Season 5",
		"VRML Season 5 Finalist",
		"VRML Season 5 Champion",
		"VRML Season 6",
		"VRML Season 6 Finalist",
		"VRML Season 6 Champion",
		"VRML Season 7",
		"VRML Season 7 Finalist",
		"VRML Season 7 Champion",
	}
	// Create the VRML groups
	for _, name := range vrmlgroups {
		groups, _, err := nk.GroupsList(ctx, name, "", nil, nil, 1, "")
		if err != nil {
			logger.WithField("err", err).Error("Group list error: %v", err)
		}
		if len(groups) == 0 {
			_, err = nk.GroupCreate(ctx, userId, name, userId, "entitlement", "VRML Badge Entitlement", "", false, map[string]interface{}{}, 1000)
			if err != nil {
				logger.WithField("err", err).Error("Group create error: %v", err)
			}
			continue
		}
		group := groups[0]
		if err := nk.GroupUpdate(ctx, group.Id, userId, name, userId, "entitlement", "VRML Badge Entitlement", "", false, map[string]interface{}{}, 1000); err != nil {
			logger.WithField("err", err).Error("Group update error: %v", err)
		}
	}
	return nil
}

// Register Indexes for the login service
func RegisterIndexes(initializer runtime.Initializer) error {
	// Register the LinkTicket Index that prevents multiple LinkTickets with the same device_id_str
	name := LinkTicketIndex
	collection := LinkTicketCollection
	key := ""                                                 // Set to empty string to match all keys instead
	fields := []string{"evrid_token", "nk_device_auth_token"} // index on these fields
	maxEntries := 10000
	indexOnly := false

	if err := initializer.RegisterStorageIndex(name, collection, key, fields, maxEntries, indexOnly); err != nil {
		return err
	}

	// Register the IP Address index for looking up user's by IP Address
	// FIXME this needs to be updated for the new login system
	name = IpAddressIndex
	collection = EvrLoginStorageCollection
	key = ""                                           // Set to empty string to match all keys instead
	fields = []string{"client_ip_address,displayname"} // index on these fields
	maxEntries = 1000000
	indexOnly = false
	if err := initializer.RegisterStorageIndex(name, collection, key, fields, maxEntries, indexOnly); err != nil {
		return err
	}
	name = EvrIDStorageIndex
	collection = GameProfileStorageCollection
	key = GameProfileStorageKey             // Set to empty string to match all keys instead
	fields = []string{"server.xplatformid"} // index on these fields
	maxEntries = 100000
	indexOnly = false
	if err := initializer.RegisterStorageIndex(name, collection, key, fields, maxEntries, indexOnly); err != nil {
		return err
	}
	// Register the DisplayName index for avoiding name collisions
	// FIXME this needs to be updated for the new login system
	name = DisplayNameIndex
	collection = EvrLoginStorageCollection
	key = ""                          // Set to empty string to match all keys instead
	fields = []string{"display_name"} // index on these fields
	maxEntries = 100000
	if err := initializer.RegisterStorageIndex(name, collection, key, fields, maxEntries, indexOnly); err != nil {
		return err
	}

	name = GhostedUsersIndex
	collection = GameProfileStorageCollection
	key = GameProfileStorageKey             // Set to empty string to match all keys instead
	fields = []string{"client.ghost.users"} // index on these fields
	maxEntries = 1000000
	if err := initializer.RegisterStorageIndex(name, collection, key, fields, maxEntries, indexOnly); err != nil {
		return err
	}

	name = ActiveSocialGroupIndex
	collection = GameProfileStorageCollection
	key = GameProfileStorageKey              // Set to empty string to match all keys instead
	fields = []string{"client.social.group"} // index on these fields
	maxEntries = 100000
	if err := initializer.RegisterStorageIndex(name, collection, key, fields, maxEntries, indexOnly); err != nil {
		return err
	}

	name = ActivePartyGroupIndex
	collection = MatchmakingStorageCollection
	key = MatchmakingConfigStorageKey // Set to empty string to match all keys instead
	fields = []string{"group_id"}     // index on these fields
	maxEntries = 100000
	if err := initializer.RegisterStorageIndex(name, collection, key, fields, maxEntries, indexOnly); err != nil {
		return err
	}

	return nil
}

func EvrApiHttpHandler(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var message interface{}
	if err := json.Unmarshal([]byte(payload), &message); err != nil {
		return "", err
	}

	logger.Info("API Service Message: %v", message)

	response, err := json.Marshal(map[string]interface{}{"message": message})
	if err != nil {
		return "", err
	}

	return string(response), nil
}
