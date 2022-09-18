# try-dandelion

Program to run many libp2p nodes with standard gossipsub or with dandelion++.

## Usage
```
git clone https://github.com/noot/go-libp2p-pubsub
cd go-libp2p-pubsub
git checkout dandelion
```

Then, somewhere else:
```
git clone https://github.com/noot/try-dandelion
cd try-dandelion
```

Modify the `go.mod` replace directive in this repo to point to your local clone of `go-libp2p-pubsub`.

Then:
```
go build
./try-dandelion --count=50 --duration=120 # without dandelion
./try-dandelion --count=50 --duration=120 --dandelion # with dandelion
```

With a lot of nodes you may need to do `ulimit -n 1000000`.