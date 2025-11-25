# 🐝 Go Swarm Simulation

> **A graphical experiment in decentralized decision-making using the Actor Model (GoAkt) and Ebitengine.**

## 🌟 Overview

**Go Swarm Simulation** is a "Game of Life on steroids" that demonstrates the power of the **Actor Model** for building concurrent, decentralized systems.

Instead of a central controller managing the state of every entity, each individual dot in this world is an autonomous **Actor** running in its own goroutine. They possess their own state, personality, and decision-making logic.

The simulation visualizes two distinct behaviors interacting in a 2D world:

* 🔴 **Red Swarm (Aggressive):** Fast-moving, jittery, and unpredictable.
* 🔵 **Blue Swarm (Consensual):** Slow-moving, stable, and drift-prone.

## 🚀 Features

* **Actor Model Architecture:** Built on [GoAkt](https://github.com/Tochemey/goakt), utilizing Protocol Buffers for high-performance message passing.
* **Real-Time Visualization:** Renders thousands of concurrent updates smoothly using [Ebitengine](https://ebitengine.org/).
* **"Push" State Sync:** Demonstrates a high-performance bridge between the Actor System and the UI thread using buffered channels (no mutex contention).
* **Behavioral Logic:** Encapsulated entirely within the actors—no central "God Object" controls movement.

## 🛠️ Architecture

The project follows a clean separation of concerns:

1.  **The Brain (Actors):** Each `Individual` is a GoAkt actor. They handle `Tick` messages to update their physics and `Push` their state to the UI.
2.  **The Protocol (Protobuf):** All messages (`Tick`, `GetState`, `ActorState`) are strictly defined in `proto` files for type safety and serialization.
3.  **The View (Ebiten):** The main game loop is a "dumb" consumer. It simply drains the update channel and renders the latest known state of every actor.

## 📦 Prerequisites

* **Go:** Version 1.22 or higher.
* **Protoc Compiler:** (Optional) Only needed if you modify the `.proto` definitions.

## 🏁 Getting Started

### 1\. Clone the Repository

```bash
git clone https://github.com/your-username/go-swarm-simulation.git
cd go-swarm-simulation
```

### 2\. Install Dependencies

Download the required Go modules (GoAkt, Ebitengine, Protobufs).

```bash
go mod tidy
```

### 3\. Run the Simulation

Launch the graphical simulation directly:

```bash
go run cmd/simulation/main.go
```

You should see a window open with **Red** and **Blue** entities moving according to their personalities\!

## 📂 Project Structure

```text
.
├── cmd/
│   └── simulation/      # Main entry point (Ebiten Game Loop)
├── internal/
│   └── individual/      # Actor logic (Behavior, State, Mailbox)
├── pb/                  # Protobuf definitions
├── scripts/             # Helper scripts (e.g., protoc generation)
└── go.mod
```

## 🧠 How It Works (Code Snippet)

The core magic happens when an actor receives a `Tick` message. It updates its own physics and **immediately pushes** a snapshot to the UI channel:

```go
// internal/individual/individual.go

func (i *Individual) Receive(ctx *actor.ReceiveContext) {
    switch ctx.Message().(type) {
    case *Tick:
        // 1. Update Physics (Private State)
        i.updatePosition()
        
        // 2. Push State to UI (Non-blocking)
        i.reportCh <- &ActorState{
            Id:        ctx.Self().Name(),
            Color:     i.Color,
            PositionX: i.X,
            PositionY: i.Y,
        }
    }
}
```

## 🤝 Contributing

Contributions are welcome\! If you want to add new behaviors (e.g., "Green" actors that chase "Red" ones) or improve the rendering performance:

1.  Fork the Project
2.  Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3.  Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4.  Push to the Branch (`git push origin feature/AmazingFeature`)
5.  Open a Pull Request

## 📜 License

Distributed under the MIT License. See `LICENSE` for more information.

-----

*Built with ❤️ using [GoAkt](https://github.com/Tochemey/goakt) and [Ebitengine](https://ebitengine.org/)*