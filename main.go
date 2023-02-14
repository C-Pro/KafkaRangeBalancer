package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/segmentio/kafka-go"
)

type Message struct {
	Topic     string
	Key       string
	Value     string
	Timestamp time.Time
}

type Producer struct {
	writer *kafka.Writer
}

type Consumer struct {
	reader *kafka.Reader
}

func NewProducer(url string) *Producer {
	urls := strings.Split(url, ",")
	return &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(urls...),
			BatchTimeout: time.Millisecond,
			RequiredAcks: kafka.RequireAll,
			Balancer:     &kafka.Hash{},
		},
	}
}

func NewConsumer(url, topics string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:        strings.Split(url, ","),
			GroupID:        "999",
			GroupTopics:    strings.Split(topics, ","),
			StartOffset:    kafka.LastOffset,
		}),
	}
}

func (p *Producer) Produce(ctx context.Context, topic, key, value string) error {
	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: []byte(value),
	}

	return p.writer.WriteMessages(ctx, msg)
}

func (c *Consumer) Consume(ctx context.Context) (chan Message, error) {
	ch := make(chan Message)
	go func() {
		<-ctx.Done()
		close(ch)
	}()

	go func() {
		for {
			m, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("reader returned %v", err)
				continue
			}

			select {
			case <-ctx.Done():
				return
			case ch <- Message{
				Topic:     m.Topic,
				Key:       string(m.Key),
				Value:     string(m.Value),
				Timestamp: m.Time,
			}:
			}
		}
	}()

	return ch, nil
}

const (
	N           = 10000
	NPartitions = 3
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	var brokers string

	flag.StringVar(&brokers, "brokers", "", `List of kafka brokers, e.g "127.0.0.1:58667,127.0.0.1:58661,127.0.0.1:58662"`)
	flag.Parse()

	if brokers == "" {
		flag.Usage()
		return
	}

	p := NewProducer(brokers)

	// maps of seen keys for each of the subscribers
	// (two topics X three partitions)
	m := map[int]map[string]map[string]any{}
	for i := 0; i < NPartitions; i++ {
		m[i] = map[string]map[string]any{}
		for _, topic := range []string{"topic1", "topic2"} {
			m[i][topic] = map[string]any{}
		}
	}

	for i := 0; i < NPartitions; i++ {
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func(i int) {
			c := NewConsumer(brokers, "topic1,topic2")
			ch, err := c.Consume(ctx)
			if err != nil {
				log.Fatalf("failed to consume: %v", err)
			}
			wg.Done()

			for {
				select {
				case msg := <-ch:
					// each goroutine writes to own (topic,i) pair,
					// so no map concurrency problems here
					m[i][msg.Topic][msg.Key] = struct{}{}
				case <-ctx.Done():
					return
				}
			}
		}(i)
		// wait until consumer is started,
		// so partition order assignment
		// will be deterministic
		wg.Wait()
	}

	start := time.Now()
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < N; i++ {
			s := strconv.Itoa(i)
			if err := p.Produce(ctx, "topic1", s, s); err != nil {
				log.Fatalf("failed to produce: %v", err)
			}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < N; i++ {
			s := strconv.Itoa(i)
			if err := p.Produce(ctx, "topic2", s, s); err != nil {
				log.Fatalf("failed to produce: %v", err)
			}
		}
	}()

	wg.Wait()
	log.Printf("Producing finished in %v", time.Since(start))
	time.Sleep(time.Second * 15) // wait for messages still in flight
	cancel()

	sumT1 := 0
	for i := 0; i < NPartitions; i++ {
		sumT1 += len(m[i]["topic1"])
	}
	if sumT1 != N {
		log.Fatalf("expected to receive %d messages from topic1, but received %d", N, sumT1)
	}

	sumT2 := 0
	for i := 0; i < NPartitions; i++ {
		sumT2 += len(m[i]["topic2"])
	}
	if sumT2 != N {
		log.Fatalf("expected to receive %d messages from topic2, but received %d", N, sumT2)
	}

	for i := 0; i < NPartitions; i++ {
		if diff := cmp.Diff(m[i]["topic1"], m[i]["topic2"]); diff != "" {
			log.Fatalf("Consumer %d partition mappings don't match:\n%s", i, diff)
		}
	}

	log.Println("ok")
}
