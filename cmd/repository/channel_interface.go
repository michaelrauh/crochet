// filepath: /Users/michaelrauh/dev/crochet/cmd/repository/channel_interface.go
package main

import (
	"github.com/rabbitmq/amqp091-go"
)

// ChannelInterface is an interface that wraps the methods we need from amqp091.Channel
type ChannelInterface interface {
	Close() error
}

// Ensure *amqp091.Channel implements ChannelInterface
var _ ChannelInterface = (*amqp091.Channel)(nil)
