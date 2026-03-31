package solver

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mercury/pkg/clients/publisher"
	"github.com/mercury/pkg/matchmaking/managers"
	"github.com/sirupsen/logrus"
	"github.com/smira/go-statsd"
)

type MMSolver interface {
	Solve(logger *logrus.Logger)
}

type mmSolver struct {
	publisherClient publisher.RMQClient
	statsdClient    *statsd.Client
	maxSolveTime    time.Duration
	mmManager       managers.MatchmakingManager
}

func NewMMSolver(
	publisherClient publisher.RMQClient,
	mmManager managers.MatchmakingManager,
	statsdClient *statsd.Client) MMSolver {
	return &mmSolver{
		publisherClient: publisherClient,
		statsdClient:    statsdClient,
		maxSolveTime:    10 * time.Minute,
		mmManager:       mmManager,
	}
}

func (s *mmSolver) Solve(logger *logrus.Logger) {
	for {
		solveID := uuid.New().String()
		ctx, cancel := context.WithTimeout(context.Background(), s.maxSolveTime)
		matched, err := s.solve(logger.WithFields(logrus.Fields{
			"solve_id": solveID,
		}), ctx)
		cancel()
		if err != nil {
			logger.WithError(err).Error("solve iteration failed")
		}
		if matched {
			time.Sleep(500 * time.Millisecond)
		} else {
			time.Sleep(5 * time.Second)
		}
	}
}

type playerNotification struct {
	serverIP   string
	serverID   string
	serverPort int
	playerID   string
}

func (s *mmSolver) solve(logger *logrus.Entry, ctx context.Context) (bool, error) {
	// gets all game servers
	gameservers, err := s.mmManager.GetGameservers()
	if err != nil {
		logger.WithError(err).Error("mmsolver failed to get game servers")
		return false, err
	}

	// gets current player counts per server, derived from assigned parties
	occupancies, err := s.mmManager.GetServerOccupancies(ctx)
	if err != nil {
		logger.WithError(err).Error("mmsolver failed to get server occupancies")
		return false, err
	}

	// gets all pending player requests
	parties, err := s.mmManager.GetPendingParties()
	if err != nil {
		logger.WithError(err).Error("mmsolver failed to get queued players")
		return false, err
	}

	// check to see if gameserver states are still valid
	for _, gs := range gameservers {
		// if player is not valid remove it from the list
		fmt.Println(gs)
	}
	// check to see if parties is still valid
	// Check with Auth that all players are still valid.
	for _, party := range parties {
		// Check that party's server is still valid, else open them up to be reassigned
		fmt.Println(party)
	}

	// build a lookup map for server info (needed for notifications)
	serverByID := make(map[string]*managers.GameserverInfo, len(gameservers))
	for _, gs := range gameservers {
		serverByID[gs.ServerID] = gs
	}

	// assigns players to servers
	toNotify := []playerNotification{}
	matchedParties := map[string]*managers.PartyInfo{}
	for _, gs := range gameservers {
		if ctx.Err() != nil {
			break
		}

		occupancy := gs.Capacity - occupancies[gs.ServerID]
		if occupancy == 0 {
			continue
		}
		for _, party := range parties {
			// skip parties that are already assigned this cycle
			if party.AssignedServerID != "" {
				continue
			}
			partySize := len(party.PlayerIDs)
			if partySize <= occupancy {
				for _, playerID := range party.PlayerIDs {
					toNotify = append(toNotify, playerNotification{
						serverIP:   gs.IPAddress,
						serverID:   gs.ServerID,
						serverPort: gs.Port,
						playerID:   playerID,
					})
				}
				party.AssignedServerID = gs.ServerID
				occupancy -= partySize
				occupancies[gs.ServerID] += partySize
				matchedParties[party.PartyID] = party
				if occupancy == 0 {
					break
				}
			}
		}
	}

	// collect matched parties for write
	updatedParties := make([]*managers.PartyInfo, 0, len(matchedParties))
	for _, party := range matchedParties {
		updatedParties = append(updatedParties, party)
	}

	// writes party assignments to database
	if err := s.mmManager.UpdateMMState(ctx, updatedParties); err != nil {
		logger.WithError(err).Error("mmsolver failed to update mm state")
		return false, err
	}

	for _, notify := range toNotify {
		response, err := s.publisherClient.SendMatchmakeNotification(ctx, notify.playerID, notify.serverID, notify.serverIP, notify.serverPort)
		if err != nil {
			// log and continue to send notifications
			continue
		}
		if response.Notified == 0 {
			// why was this guy not notified?
			// offline before deregistering?
			// bad pubsub?
			continue
		}
	}
	return len(matchedParties) > 0, nil
}
