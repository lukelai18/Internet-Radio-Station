Milestone Design Document

Author: Zekai Lai

## Components

Language: Go

- Server
  - main.go
- Client
  - main.go
- pkg
  - Protocol
    - Protocol.go

1. How will your server use threads/goroutines? In other words, when are they created, and for what purpose?
    My server use goroutines for concurrency. And my server creates goroutines after a client connects to server. In addition, these goroutines are used to handle interaction between server and different clients and handle the change of client stations.

2. What data does your server need to store for each client?
    It should store the connection information of clients, udp port number of clients and the station ID.

3. What data does your server need to store for each station?
    It should store the station ID, the media file inside the station, and the clients listening to this station.

4. What synchronization primitives (channels, mutexes, condition variables, etc.) will you need to have      threads/goroutines communicate with each other and protect shared data?
    1. I think I'll need mutexes to protect shared data, for example, when mutiple clients connect to the same station and trying to write data to the station, I think we need to use mutex to avoid they are modifying the same data. 
    2. I think I also need channels, which support goroutines send data between each other.

5. What happens on the server (in terms of the threads/data structures/etc you described in (1)-(4)) when a client changes stations?
    I think the client will send a request to server firstly. When the server agrees this request. It'll use mutex to lock the old station and new station. Then the server will remove the client from the old station. After that, the server will update the station ID of the client, and add the client to the connectef list of new station. After finish the above process, the server will unlock the mutex in the old and new station.