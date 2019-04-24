// Package meetingbot implements a Service which echoes back !commands.
package meetingbot

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/matrix-org/go-neb/types"
	"github.com/matrix-org/gomatrix"
)

// ServiceType of the meetingbot service
const ServiceType = "meetingbot"

// Service represents the meetingbot service. It has no Config fields.
type Service struct {
	types.DefaultService
}

var pendingSlice []string
var doneSlice []string
var currentUser string
var meetingChair = ""
var regexpAll = regexp.MustCompile(".*")

var mutex sync.Mutex

func reset() {
	pendingSlice = nil
	doneSlice = nil
	meetingChair = ""
}

// Must hold mutex before calling this!
func markPresent(userID string) {
	var present = false
	for i := 0; i < len(pendingSlice); i++ {
		if pendingSlice[i] == userID {
			present = true
			break
		}
	}
	if !present {
		pendingSlice = append(pendingSlice, userID)
	}
	return
}

// Commands supported:
//    !rollcall
// Responds with a notice of "meeting started"
//    !present
// Adds user to meeting queue
//    !next
// Pings the next user in queue for their turn
func (e *Service) Commands(cli *gomatrix.Client) []types.Command {
	return []types.Command{
		types.Command{
			Path: []string{"rollcall"},
			Command: func(roomID, userID string, args []string) (interface{}, error) {
				mutex.Lock()
				defer mutex.Unlock()

				if meetingChair != "" {
					return &gomatrix.TextMessage{"m.text", string("Meeting already in progress")}, nil
				}
				meetingChair = userID
				return &gomatrix.TextMessage{"m.text", string("Hello @room, Welcome to meeting, to mark yourself present, say !present, meeting chair can run !next command to start meeting.")}, nil
			},
		},
		types.Command{
			Path: []string{"present"},
			Command: func(roomID, userID string, args []string) (interface{}, error) {
				mutex.Lock()
				defer mutex.Unlock()

				markPresent(userID)

				return nil, nil
			},
		},
		types.Command{
			Path: []string{"next"},
			Command: func(roomID, userID string, args []string) (interface{}, error) {
				mutex.Lock()
				defer mutex.Unlock()

				if userID != meetingChair {
					return &gomatrix.TextMessage{"m.text", string("To avoid confusion, only the chair may progress")}, nil
				}

				if len(pendingSlice) > 0 {
					currentUser = pendingSlice[0]
					pendingSlice = pendingSlice[1:]
					doneSlice = append(doneSlice, currentUser)

					var nextUser = "Silence!"
					if len(pendingSlice) > 0 {
						nextUser = pendingSlice[0]
					}
					var message = fmt.Sprintf("%s's turn, Followed by %s", currentUser, nextUser)

					if len(pendingSlice) == 0 {
						reset()
					}

					return &gomatrix.TextMessage{"m.text", message}, nil
				}

				return &gomatrix.TextMessage{"m.text", string("Meeting is over, thanks for attending!")}, nil
			},
		},
		types.Command{
			Path: []string{"debug"},
			Command: func(roomID, userID string, args []string) (interface{}, error) {
				mutex.Lock()
				defer mutex.Unlock()

				fmt.Printf("chair: %s - pending: %v - done: %v",
					meetingChair, pendingSlice, doneSlice)
				return nil, nil
			},
		},
	}
}

func (s *Service) Expansions(cli *gomatrix.Client) []types.Expansion {
	return []types.Expansion{
		types.Expansion{
			Regexp: regexpAll,
			Expand: func(roomID, userID string, issueKeyGroups []string) interface{} {
				mutex.Lock()
				defer mutex.Unlock()

				if meetingChair == "" {
					return nil // no meeting in progress
				}

				for _, doneAttendee := range doneSlice {
					if doneAttendee == userID {
						return nil // user already done
					}
				}

				for _, attendee := range pendingSlice {
					if attendee == userID {
						return nil // user queued
					}
				}

				if userID != s.ServiceUserID() && userID != meetingChair {
					pendingSlice = append(pendingSlice, userID)
				}

				return nil
			},
		},
	}
}

func init() {
	types.RegisterService(func(serviceID, serviceUserID, webhookEndpointURL string) types.Service {
		mutex.Lock()
		defer mutex.Unlock()

		return &Service{
			DefaultService: types.NewDefaultService(serviceID, serviceUserID, ServiceType),
		}
	})
}
