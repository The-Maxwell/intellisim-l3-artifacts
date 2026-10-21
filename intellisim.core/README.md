# IntelliSim - Agent-Based Model Simulator

A comprehensive platform for creating and visualizing agent-based models through natural language conversations powered by LLMs.

## Overview

IntelliSim combines the power of Large Language Models (LLMs) with agent-based modeling to create a unique simulation platform where users can:

- **Generate models through conversation**: Describe your simulation in natural language
- **Visualize spatial simulations**: Real-time canvas rendering with WebSocket updates
- **Configure LLM strategies**: Choose how AI controls your agents (from rule-based to full LLM control)
- **Run complex simulations**: Support for rules, interactions, memory, and state machines

## Architecture

### Backend (Go)
- **API Server**: RESTful API with Echo framework
- **Model Generation**: LLM-powered model creation from natural language
- **Simulation Engine**: Multiple execution patterns (6 LLM integration strategies)
- **Rule Engine**: Condition-action system for agent behavior
- **Data Persistence**: MongoDB for models, simulations, and chat history
- **Real-time Updates**: WebSocket broadcasting of simulation steps

### Frontend (Vue 3)
- **Chat Interface**: Conversational model creation
- **Visual Editor**: Review and refine generated models
- **Canvas Visualization**: Real-time rendering of spatial simulations
- **Dashboard**: Monitor agent properties and simulation progress

## Features

### 🤖 LLM-Powered Model Generation
- Describe models in natural language
- Iterative refinement through conversation
- Automatic generation of agents, rules, interactions, and environments
- Support for spatial and non-spatial models

### 📊 Comprehensive ABM Support
- **Agent Types**: Archetypes and specific agents
- **Properties**: Dynamic typed properties with validation
- **Rules**: Condition-action system with priorities and cooldowns
- **Interactions**: Agent-agent, agent-shape, and global interactions
- **Memory**: Agents remember past events and decisions
- **States**: State machine support for complex behaviors

### 🎮 Multiple LLM Strategies
1. **None**: Pure rule-based execution
2. **Individual Full**: LLM controls each agent completely
3. **Archetype Full**: LLM provides strategies for agent types
4. **Individual Decision**: LLM makes decisions, rules execute them
5. **Archetype Decision**: LLM makes archetype decisions, rules execute
6. **Hybrid**: Mix of strategies

### 🎨 Real-time Visualization
- Canvas rendering for spatial models
- WebSocket-based live updates
- Agent tracking and property monitoring
- Environment shape rendering

## Quick Start

### Prerequisites
- Go 1.21+
- Node.js 18+
- MongoDB 4.4+
- OpenAI API key

### Installation

#### 1. Clone and Setup Backend

```bash
# Install Go dependencies
go mod download

# Configure environment
cp .env.example .env
# Edit .env with your MongoDB URI and OpenAI API key

# Build and run
go build -o intelli-sim.exe ./cmd/intelli-sim
./intelli-sim.exe
```

#### 2. Setup Frontend

```bash
cd ui
npm install
npm run dev
```

#### 3. Access the Application

Open your browser to `http://localhost:3000`

### Using the Startup Script

**Windows:**
```bash
start-dev.bat
```

**Linux/macOS:**
```bash
./start-dev.sh
```

## Usage Example

### 1. Create a Chat
Click "New Chat" and name it (e.g., "Evacuation Model")

### 2. Describe Your Model
```
Create an evacuation simulation with 100 people trying to exit a building.
The building is 200x150 units with 2 exit doors on opposite sides.
People move at different speeds and may panic near fire zones.
```

### 3. Review Generated Model
Check the right panel for:
- Agent types (People, Doors, Fire zones)
- Properties (speed, panic_level, health)
- Rules (movement, panic behavior, energy depletion)
- Interactions (agent-door, agent-fire)
- Environment (spatial layout, shapes)

### 4. Refine Model (Optional)
```
Add obstacles in the center of the room
Increase population to 200
Add a smoke spread mechanic
```

### 5. Create and Run Simulation
- Click "Create Simulation"
- Set steps (e.g., 100)
- Watch real-time visualization

## Project Structure

```
intelli.sim.core/
├── api/                      # HTTP handlers
├── cmd/intelli-sim/         # Application entry point
├── data/                    # Data layer
│   ├── chats/              # Chat documents
│   ├── models/             # Model documents
│   └── simulations/        # Simulation documents
├── modules/                 # Business logic
│   ├── chats/              # Chat management
│   ├── llm/                # LLM integration
│   ├── models/             # Model generation & mapping
│   └── simulations/        # Simulation engines
│       ├── engine.go           # Original engine
│       ├── engine2.go          # Optimized engine
│       ├── engine_enhanced.go  # 6-pattern engine
│       ├── rule_engine.go      # Rule evaluation
│       └── types.go            # Shared types
├── pkg/                     # Shared utilities
│   ├── httpserver/         # HTTP server setup
│   └── mongodb/            # Database connection
├── ui/                      # Vue 3 Frontend
│   ├── src/
│   │   ├── api/            # API client
│   │   ├── components/     # Reusable components
│   │   ├── composables/    # Vue composables
│   │   ├── types/          # TypeScript types
│   │   ├── views/          # Page components
│   │   ├── App.vue         # Root component
│   │   ├── main.ts         # Entry point
│   │   └── router.ts       # Routes
│   ├── package.json
│   └── vite.config.ts
├── .env                     # Environment variables
├── go.mod                   # Go dependencies
├── SETUP.md                 # Detailed setup guide
└── README.md               # This file
```

## API Documentation

### REST Endpoints

**Chats**
- `GET /api/chats` - List all chats
- `POST /api/chats` - Create chat
- `GET /api/chats/:id` - Get chat
- `DELETE /api/chats/:id` - Delete chat
- `GET /api/chats/:id/messages` - Get messages
- `POST /api/chats/:id/messages` - Send message

**Models**
- `GET /api/models` - List models
- `POST /api/models/generate` - Generate from chat
- `POST /api/models/:id/update` - Update model
- `GET /api/models/:id` - Get model
- `DELETE /api/models/:id` - Delete model

**Simulations**
- `GET /api/simulations` - List simulations
- `POST /api/simulations` - Create simulation
- `POST /api/simulations/:id/start` - Start simulation
- `GET /api/simulations/:id` - Get simulation
- `DELETE /api/simulations/:id` - Delete simulation

**WebSocket**
- `WS /ws/simulations/:id` - Real-time simulation updates

## Configuration

### Backend (.env)
```env
MONGODB_URI=mongodb://localhost:27017
MONGODB_DATABASE=intellisim
OPENAI_API_KEY=sk-...
PORT=8080
```

### Frontend (ui/.env)
```env
VITE_API_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080
```

## Technology Stack

### Backend
- **Language**: Go 1.21
- **Framework**: Echo (HTTP), Uber FX (DI)
- **Database**: MongoDB
- **LLM**: OpenAI GPT-4
- **Real-time**: Gorilla WebSocket

### Frontend
- **Framework**: Vue 3 + TypeScript
- **Build Tool**: Vite
- **UI Library**: PrimeVue
- **Styling**: TailwindCSS
- **HTTP Client**: Axios
- **Routing**: Vue Router

## Development

### Running Tests

```bash
go test ./...
```

### Building for Production

**Backend:**
```bash
go build -o intelli-sim ./cmd/intelli-sim
```

**Frontend:**
```bash
cd ui
npm run build
```

## Use Cases

- **Evacuation Planning**: Model emergency scenarios
- **Ecology**: Predator-prey dynamics
- **Epidemiology**: Disease spread simulation
- **Economics**: Market behavior modeling
- **Urban Planning**: Traffic and crowd flow
- **Social Networks**: Information diffusion
- **Supply Chain**: Logistics optimization

## Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## Documentation

- [SETUP.md](SETUP.md) - Complete setup guide
- [ui/README.md](ui/README.md) - Frontend documentation
- [ui/QUICKSTART.md](ui/QUICKSTART.md) - Quick start guide

## License

Copyright (c) 2026 Vladyslav Pryshchepa.

Licensed under the [PolyForm Noncommercial License 1.0.0](LICENSE) — free to use, modify and
share for **noncommercial purposes** such as scientific research, academic study and teaching.

**Commercial use requires a separate license.** Contact vlad.prishhepa@gmail.com.

## Support

For issues, questions, or feature requests, please open an issue on GitHub.

## Acknowledgments

- OpenAI for LLM capabilities
- The ABM research community
- Open source contributors

---

Built with ❤️ for the agent-based modeling community
