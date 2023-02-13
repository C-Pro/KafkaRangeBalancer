# Experiment with GroupBalancer

TL/DR: it works.

The purpose is to verify that we can split message stream between several consumers in ConsumerGroup in such a way
that each consumer will receive some subset of keys and this set will be stable between topics.

For example if 5 consumers subscribe to "balances" and "orders" topics we want to be sure that we would not get a situation when
consumer 1 gets balance update of user 1 and consumer 2 gets order update of user 1.
We want user 1 data always to land to one consumer in consumer group.

To achieve this we use Hash message->partition mapper in the producer and RangeGroupBalancer consumer partition mapper in consumer.

## Run it

For some reason with provided docker-compose for kafka I got this error:

```
2023/02/13 17:12:46 failed to produce: [3] Unknown Topic Or Partition: the request is for a topic or partition that does not exist on this broker
```

I suspect topics were not created properly despite magic `KAFKA_CREATE_TOPICS="topic1:3:1,topic2:3:1"` env variable. When something does not work in Kafka I just try to do the same in Redpanda.

With Redpanda it works out of the box:

```
$ rpk container start -n 3
Starting cluster
Waiting for the cluster to be ready...
  NODE ID  ADDRESS
  0        127.0.0.1:58667
  1        127.0.0.1:58661
  2        127.0.0.1:58662

Cluster started! You may use rpk to interact with it. E.g:

  rpk cluster info --brokers 127.0.0.1:58667,127.0.0.1:58661,127.0.0.1:58662

You may also set an environment variable with the comma-separated list of broker addresses:

  export REDPANDA_BROKERS="127.0.0.1:58667,127.0.0.1:58661,127.0.0.1:58662"
  rpk cluster info
$ REDPANDA_BROKERS="127.0.0.1:58667,127.0.0.1:58661,127.0.0.1:58662"
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
