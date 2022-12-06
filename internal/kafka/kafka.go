package kafka

import (
	"crypto/tls"
	"fmt"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"strings"
	"time"
)

type KafkaAuthCredentials struct {
	Username string
	Password string
}

type KafkaReaderOption func(*kafka.ReaderConfig)

func WithBrokers(brokers []string) KafkaReaderOption {
	return func(h *kafka.ReaderConfig) {
		h.Brokers = brokers
	}
}

func WithGroupID(GroupID string) KafkaReaderOption {
	return func(h *kafka.ReaderConfig) {
		h.GroupID = GroupID
	}
}

func WithTopic(Topic string) KafkaReaderOption {
	return func(h *kafka.ReaderConfig) {
		h.Topic = Topic
	}
}

func WithDialer(Dialer *kafka.Dialer) KafkaReaderOption {
	return func(h *kafka.ReaderConfig) {
		h.Dialer = Dialer
	}
}

func WithPartition(Partition int) KafkaReaderOption {
	return func(h *kafka.ReaderConfig) {
		h.Partition = Partition
	}
}

func NewReaderConfig(opts ...KafkaReaderOption) *kafka.Reader {
	const (
		kafkaMinBytes    = 10
		kafkaMaxBytes    = 10e6
		kafkaMaxAttempts = 16
	)

	readerConfig := &kafka.ReaderConfig{
		MinBytes:    kafkaMinBytes,
		MaxBytes:    kafkaMaxBytes,
		MaxAttempts: kafkaMaxAttempts,
	}

	for _, opt := range opts {
		opt(readerConfig)
	}

	return kafka.NewReader(*readerConfig)
}

func NewDialer(creds, dialerTimeout string) (*kafka.Dialer, error) {
	mechanism, err := parseKafkaSaslPlain(creds)
	if err != nil {
		return nil, fmt.Errorf("failed to parse consumer credentials: %w", err)
	}

	timeout, err := time.ParseDuration(dialerTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timeout duration: %w", err)
	}

	return &kafka.Dialer{
		Timeout:       timeout,
		DualStack:     true,
		TLS:           &tls.Config{},
		SASLMechanism: mechanism,
	}, nil
}

func parseKafkaSaslPlain(creds string) (*plain.Mechanism, error) {
	credsSplit := strings.SplitN(creds, ":", 2)
	if len(credsSplit) == 1 {
		return nil, fmt.Errorf("failed to parse credentials")
	}
	return &plain.Mechanism{
		Username: credsSplit[0],
		Password: credsSplit[1],
	}, nil
}
