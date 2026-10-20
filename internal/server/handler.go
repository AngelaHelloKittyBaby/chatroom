package server

import (
	"fmt"
	"log"
	"strings"
	"time"

	"chatroom/internal/protocol"
	"chatroom/internal/validator"
)

func ReadLoop(client *Client, msgValidator validator.MessageValidator, limiter RateLimiter, readTimeout time.Duration) {
	defer client.Room().Leave(client)

	for {
		client.SetReadDeadline(time.Now().Add(readTimeout))

		msg, err := protocol.ReadMessage(client.Conn())
		if err != nil {
			if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
				log.Printf("[%s] read timeout", client.Name())
			} else {
				log.Printf("[%s] read failed: %v", client.Name(), err)
			}
			return
		}

		if msg.Type == protocol.TypeChat && limiter != nil && !limiter.Allow(client.ID()) {
			client.Send(protocol.NewErrorMessage("sending too fast, please slow down"))
			continue
		}

		processMessage(client, msg, msgValidator)
	}
}

func processMessage(client *Client, msg *protocol.Message, msgValidator validator.MessageValidator) bool {
	switch msg.Type {
	case protocol.TypePong:
		client.SignalPong()
		return true

	case protocol.TypeChat:
		content := strings.TrimSpace(msg.Content)
		if strings.HasPrefix(content, "/") {
			handleCommand(client, content, msgValidator)
			return true
		}

		content = validator.TrimContent(msg.Content)
		if msgValidator != nil {
			if err := msgValidator.ValidateContent(content); err != nil {
				client.Send(protocol.NewErrorMessage(fmt.Sprintf("invalid message: %v", err)))
				return true
			}
		}

		if client.server != nil && client.server.redis != nil {
			if err := client.server.enqueue(client.Room().Name(), protocol.NewChatMessage(client.Name(), content)); err != nil {
				client.Send(protocol.NewErrorMessage("Message queue unavailable, please retry"))
				return true
			}
		} else {
			client.Room().Broadcast(protocol.NewChatMessage(client.Name(), content))
		}
		client.IncrRecv()
		return true
	}

	log.Printf("[%s] message not handled: type=%s", client.Name(), msg.Type)
	return false
}

func handleCommand(client *Client, content string, msgValidator validator.MessageValidator) {
	parts := strings.Fields(content)
	if len(parts) == 0 {
		return
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "/quit":
		client.Room().Leave(client)

	case "/rank":
		if client.server == nil {
			client.Send(protocol.NewErrorMessage("Redis is disabled"))
			return
		}
		client.server.showRanking(client, args)
	case "/profile":
		if client.server != nil {
			client.server.showProfile(client)
		}
	case "/help":
		client.Send(protocol.NewSystemMessage("/msg <username> <message> | /users | /rank [1-100] | /profile | /quit"))
	case "/users":
		client.Send(protocol.NewUsersMessage(client.Room().UserNames()))

	case "/msg", "/whisper", "/w":
		handlePrivateMsg(client, args, msgValidator)

	default:
		client.Send(protocol.NewErrorMessage(fmt.Sprintf("unknown command: %s", cmd)))
	}
}

func handlePrivateMsg(client *Client, args []string, msgValidator validator.MessageValidator) {
	if len(args) < 2 {
		client.Send(protocol.NewErrorMessage("usage: /msg <username> <message>"))
		return
	}

	targetName := args[0]
	privateContent := strings.Join(args[1:], " ")

	if msgValidator != nil {
		if err := msgValidator.ValidateContent(privateContent); err != nil {
			client.Send(protocol.NewErrorMessage(fmt.Sprintf("invalid private message: %v", err)))
			return
		}
	}

	if targetName == client.Name() {
		client.Send(protocol.NewErrorMessage("cannot send private message to yourself"))
		return
	}

	target, found := client.Room().FindByName(targetName)
	if !found {
		client.Send(protocol.NewErrorMessage(fmt.Sprintf("user %q is not online", targetName)))
		return
	}

	privateMsg := protocol.NewPrivateMessage(client.Name(), targetName, privateContent)
	if client.server != nil && client.server.redis != nil {
		if err := client.server.enqueue(client.Room().Name(), privateMsg); err != nil {
			client.Send(protocol.NewErrorMessage("Message queue unavailable, please retry"))
		}
		return
	}
	if !target.Send(privateMsg) {
		client.Send(protocol.NewErrorMessage(fmt.Sprintf("cannot send to %s", targetName)))
		return
	}

	client.Send(protocol.NewPrivateMessage(client.Name(), targetName, privateContent))
	log.Printf("[%s] -> [%s]: %s", client.Name(), targetName, privateContent)
}

func WriteLoop(client *Client, writeTimeout time.Duration) {
	for {
		select {
		case <-client.Done():
			return
		case msg, ok := <-client.SendCh():
			if !ok {
				return
			}
			client.SetWriteDeadline(time.Now().Add(writeTimeout))
			if _, err := protocol.WriteMessage(client.Conn(), msg); err != nil {
				log.Printf("[%s] send failed: %v", client.Name(), err)
				return
			}
		}
	}
}
