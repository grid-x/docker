# docker

docker is a lightweight docker libary that uses only the golang standard libary.
This library can be used for basic operations like start, stop, create and delete containers.

Example: create, start, stop and delete a container

```golang
package main

import (
    "log"
    "time"

    "github.com/grid-x/docker"
)

func main() {
    dc := docker.NewClient("/var/run/docker.sock")

    const container = "gridx-docker-test"
    log.Printf("create container %s", container)
    exposedPorts := []string{"80/tcp"}
    cID, err := dc.CreateContainer(container, "nginxdemos/hello:plain-text", nil, exposedPorts, nil)
    if err != nil {
        log.Println(err)
    }
    defer func() {
        log.Printf("delete container %s", container)
        if err := dc.DeleteContainer(cID); err != nil {
            log.Println(err)
        }
    }()

    log.Printf("start container %s", container)
    if err := dc.StartContainer(cID); err != nil {
        log.Println(err)
    }
    defer func() {
        log.Printf("stop container %s", container)
        if err := dc.StopContainer(cID); err != nil {
            log.Println(err)
        }
    }()

    // simulating a running program. During this time we can check if
    // everything was started by entering docker network ls and
    // docker container ps.
    time.Sleep(time.Second * 60)
}
```

Example: Create a container from a container and connect both containers via a docker network

```golang
package main

import (
    "fmt"
    "log"
    "os"
    "time"

    "github.com/grid-x/docker"
)

func main() {
    dc := docker.NewClient("/var/run/docker.sock")

    // get containerID e.g.:
    // docker run --rm -it ubuntu:latest
    // root@ef3e8b0e9c4c:/# echo $HOSTNAME
    // ef3e8b0e9c4c
    myID := os.Getenv("HOSTNAME")

    log.Println("myID", myID)

    var i int

    network := fmt.Sprintf("test_net_%d", i)
    log.Printf("create network %s", network)
    nwID, err := dc.CreateNetwork(network)
    if err != nil {
        log.Fatal(err)
    }
    log.Println("networkID", nwID)
    defer dc.DeleteNetwork(nwID)

    container := fmt.Sprintf("test_%d", i)
    log.Printf("create container %s", container)
    exposedPorts := []string{"80/tcp"}
    cID, err := dc.CreateContainer(container, "nginxdemos/hello:plain-text", nil, exposedPorts, nil)
    if err != nil {
        log.Println(err)
    }
    defer dc.DeleteContainer(cID)

    if err := dc.ConnectNetwork(myID, nwID, nil); err != nil {
        log.Println(err)
    }
    defer dc.DisconnectNetwork(myID, nwID)

    if err := dc.ConnectNetwork(cID, nwID, nil); err != nil {
        log.Println(err)
    }
    defer dc.DisconnectNetwork(cID, nwID)

    if err := dc.StartContainer(cID); err != nil {
        log.Println(err)
    }
    defer dc.StopContainer(cID)

    // simulating a running program. During this time we can check if
    // everything was started by entering docker network ls and
    // docker container ps.
    time.Sleep(time.Second * 60)
}

```
