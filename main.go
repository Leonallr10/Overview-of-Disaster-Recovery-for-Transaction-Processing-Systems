package main

import (
    "fmt"
    "math/rand"
    "sync"
    "time"
)

const (
    Preparing  = "PREPARING"
    Prepared   = "PREPARED"
    Committing = "COMMITTING"
    Committed  = "COMMITTED"
    Aborting   = "ABORTING"
    Aborted    = "ABORTED"
    Failure    = "FAILURE" // New state to simulate failure
)

type Participant struct {
    ID              string
    State           string
    response        chan string
    ChandyLamport   map[string]int // Vector clock for Chandy-Lamport snapshot algorithm
    IsCoordinator   bool           // Flag to indicate if the participant is the coordinator
    ElectionChannel chan string    // Channel for election messages
    Coordinator     *Coordinator   // Reference to the current coordinator
}

func NewParticipant(id string) *Participant {
    return &Participant{
        ID:              id,
        State:           Preparing,
        response:        make(chan string),
        IsCoordinator:   false,
        ElectionChannel: make(chan string),
        Coordinator:     nil,
    }
}

func (p *Participant) StartTransaction() {
    fmt.Printf("[%s] Participant %s: Transaction started\n", time.Now().Format("15:04:05"), p.ID)
    p.State = Preparing
}

func (p *Participant) InitChandyLamportClock(participants []*Participant) {
    p.ChandyLamport = make(map[string]int)
    for _, participant := range participants {
        p.ChandyLamport[participant.ID] = 0
    }
}

func (p *Participant) ChandyLamportSnapshot(coordinator *Coordinator) {
    for _, participant := range coordinator.participants {
        if participant.ID != p.ID {
            p.ChandyLamport[participant.ID] = participant.ChandyLamport[participant.ID]
        }
    }
}

func (p *Participant) RingElection(coordinator *Coordinator) {
    nextID := (len(coordinator.participants) + 1) % len(coordinator.participants)
    go func() {
        coordinator.participants[nextID].ElectionChannel <- p.ID
    }()
}

func (p *Participant) Prepare(coordinator *Coordinator, wg *sync.WaitGroup) {
    defer wg.Done() // Decrement the WaitGroup counter when this function completes

    // Perform Chandy-Lamport snapshot
    p.ChandyLamportSnapshot(coordinator)

    // Perform election if the participant is not the coordinator
    if !p.IsCoordinator {
        p.RingElection(coordinator)
    }

    fmt.Printf("[%s] Participant %s: Preparing...\n", time.Now().Format("15:04:05"), p.ID)
    time.Sleep(time.Duration(rand.Intn(3)) * time.Second)
    if rand.Intn(10) < 8 {
        p.State = Failure
        fmt.Printf("[%s] Participant %s: Failure occurred during preparation\n", time.Now().Format("15:04:05"), p.ID)
        return // Exit the function if there's a failure
    }
    p.State = Prepared
    fmt.Printf("[%s] Participant %s: Prepared\n", time.Now().Format("15:04:05"), p.ID)
}

func (p *Participant) CommitOrAbort() {
    switch p.State {
    case Failure:
        fmt.Printf("[%s] Participant %s: Recovery started\n", time.Now().Format("15:04:05"), p.ID)
        time.Sleep(2 * time.Second) // Simulate recovery time
        p.Prepare(p.Coordinator, nil)
    case Prepared:
        select {
        case decision, ok := <-p.response:
            if !ok { // Check if the response channel is closed
                return // Exit the function if the channel is closed
            }
            if decision == "COMMIT" {
                p.State = Committing
                fmt.Printf("[%s] Participant %s: Committing...\n", time.Now().Format("15:04:05"), p.ID)
                time.Sleep(time.Duration(rand.Intn(3)) * time.Second)
                p.State = Committed
                fmt.Printf("[%s] Participant %s: Committed\n", time.Now().Format("15:04:05"), p.ID)
            } else {
                p.State = Aborting
                fmt.Printf("[%s] Participant %s: Aborting...\n", time.Now().Format("15:04:05"), p.ID)
                time.Sleep(time.Duration(rand.Intn(3)) * time.Second)
                p.State = Aborted
                fmt.Printf("[%s] Participant %s: Aborted\n", time.Now().Format("15:04:05"), p.ID)
            }
        case <-time.After(5 * time.Second):
            fmt.Printf("[%s] Participant %s: Timeout occurred\n", time.Now().Format("15:04:05"), p.ID)
            p.State = Aborted
            fmt.Printf("[%s] Participant %s: Aborted due to timeout\n", time.Now().Format("15:04:05"), p.ID)
        }
    }
}

func (p *Participant) ReceiveDecision(decision string) {
    p.response <- decision
}

type Coordinator struct {
    participants []*Participant
}

func NewCoordinator() *Coordinator {
    return &Coordinator{
        participants: make([]*Participant, 0),
    }
}

func (c *Coordinator) AddParticipant(p *Participant) {
    c.participants = append(c.participants, p)
}

func (c *Coordinator) StartTransaction() {
    fmt.Printf("[%s] Coordinator: Transaction started\n", time.Now().Format("15:04:05"))
    for _, p := range c.participants {
        p.StartTransaction()
    }
}

func (c *Coordinator) Prepare() {
    fmt.Printf("[%s] Coordinator: Preparing...\n", time.Now().Format("15:04:05"))
    var wg sync.WaitGroup       // Initialize WaitGroup
    wg.Add(len(c.participants)) // Add the number of participants to the WaitGroup counter
    for _, p := range c.participants {
        go p.Prepare(c, &wg) // Pass WaitGroup pointer to each participant's Prepare function
    }
    wg.Wait() // Wait for all participants to finish
}

func (c *Coordinator) CommitOrAbort(decision string) {
    fmt.Printf("[%s] Coordinator: Received decision %s\n", time.Now().Format("15:04:05"), decision)
    var wg sync.WaitGroup
    for _, p := range c.participants {
        wg.Add(1)
        go func(p *Participant) {
            defer wg.Done()
            if p.State == Prepared {
                p.ReceiveDecision(decision)
                p.CommitOrAbort()
            } else if p.State == Failure {
                fmt.Printf("[%s] Participant %s: Skipping due to failure\n", time.Now().Format("15:04:05"), p.ID)
            }
        }(p)
    }
    wg.Wait()
}

func main() {
    rand.Seed(time.Now().UnixNano())

    // Create coordinator and participants
    coordinator := NewCoordinator()
    participant1 := NewParticipant("P1")
    participant2 := NewParticipant("P2")
    participant3 := NewParticipant("P3")

    // Register participants with the coordinator
    coordinator.AddParticipant(participant1)
    coordinator.AddParticipant(participant2)
    coordinator.AddParticipant(participant3)

    // Initialize Chandy-Lamport clock for participants
    for _, participant := range coordinator.participants {
        participant.InitChandyLamportClock(coordinator.participants)
    }

    // Start transaction
    coordinator.StartTransaction()

    // Phase 1: Prepare
    coordinator.Prepare()

    // Simulate decision
    decision := "COMMIT" // or "ABORT"

    // Phase 2: Commit or Abort
    coordinator.CommitOrAbort(decision)
}
