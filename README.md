# Experiment with GroupBalancer

TL/DR: it works.

The purpose is to verify that we can split message stream between several consumers in ConsumerGroup in such a way
that each consumer will receive some subset of keys and this set will be stable between topics.

For example if 5 consumers subscribe to "balances" and "orders" topics we want to be sure that we would not get a situation when
consumer 1 gets balance update of user 1 and consumer 2 gets order update of user 1.
We want user 1 data always to land to one consumer in consumer group.

To achieve this we use Hash message->partition mapper in the producer and RangeGroupBalancer consumer partition mapper in consumer.

## Run it

### Kafka

The cluster and  topics setup are hardcoded in docker-compose. To run it you need first to change advertized listener IPs to your network interface IP address (localhost won't work).

```
$ docker-compose up -d
$ go run main.go -brokers 127.0.0.1:9093,127.0.0.1:9094,127.0.0.1:9095
2023/02/14 12:34:59 Producing finished in 19.454449084s
2023/02/14 12:35:14 ok
```

### Redpanda

With Redpanda you can easily create cluster and topics with couple of `rpk` commands.

```
$ rpk container start -n 3
Starting cluster
Waiting for the cluster to be ready...
  NODE ID  ADDRESS
  0        127.0.0.1:58667
  1        127.0.0.1:58661
  2        127.0.0.1:58662
  ...

$ export REDPANDA_BROKERS="127.0.0.1:58667,127.0.0.1:58661,127.0.0.1:58662"
$ rpk topic create topic1 -p 3 -r 1
TOPIC   STATUS
topic1  OK

$ rpk topic create topic2 -p 3 -r 1
TOPIC   STATUS
topic2  OK

$ go run main.go -brokers 127.0.0.1:58667,127.0.0.1:58661,127.0.0.1:58662
2023/02/13 17:35:50 Producing finished in 19.267179125s
2023/02/13 17:35:55 ok
```
