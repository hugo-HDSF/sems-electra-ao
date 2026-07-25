# SEMS - Smart Energy Management System

SEMS is an in-memory, deterministic simulation engine for managing EV charging stations with Battery Energy Storage Systems (BESS). It dynamically allocates power to connected EVs using a two-level proportional distribution algorithm while enforcing hardware limits, requested power constraints, and grid limits.

## Architecture

*   **Hexagonal Architecture:** The codebase is organized into domain, service, and API layers to keep core business logic pure and decoupled from infrastructure.
*   **Two-Level Proportional Allocation:** Power is proportionally split first among active EVSEs (based on total connector demand), and then proportionally among the active connectors within each EVSE, capped at limits.
*   **BESS Integration:** Integrates a battery that charges on spare grid power and discharges to boost available power during high demand, while strictly enforcing a 10% SoC floor.
*   **Time Simulation:** Time is advanced synthetically via a `Tick` API to update SoC and automatically disconnect fully charged vehicles in a deterministic manner.

## Getting Started

### Prerequisites
- Docker & Docker Compose (Go is not required as the environment is fully containerized)
### Run via Docker (Recommended)

This approach ensures the application works identically on all computers without needing to install Go.

```bash
# Run all tests inside a Docker container
make test

# Build and run the entire stack (with live logs attached)
make build

# Build and run the entire stack in the background (detached)
make build-d
```



## API Documentation

Swagger UI is available at `http://localhost:8080/swagger/`.
The raw OpenAPI 3.0 specification can be found in `swagger/openapi.yaml`.

### Domain Logic & Architecture Schema

The codebase is strictly separated into three layers. This architecture deliberately isolates the concurrent API threads and mutex locking (Service Layer) entirely away from the pure, deterministic mathematical models (Domain Layer).

```mermaid
flowchart TD
    %% Define API Layer
    subgraph API["API Layer (internal/api)"]
        H["HTTP Handlers"]
        SSE["Server-Sent Events"]
    end

    %% Define Service Layer
    subgraph Service["Service Layer (internal/service)"]
        SC["SiteController (Orchestrator)"]
        Mut["Mutations (Connect, Disconnect, Tick)"]
    end

    %% Define Domain Layer
    subgraph Domain["Domain Layer (internal/domain)"]
        Alloc["Power Allocation Logic"]
        State["Station State (EVSEs, BESS, Sessions)"]
    end

    %% Data Flow
    Client((Client)) -->|HTTP Requests| H
    H -->|Invoke| SC
    
    SC -->|1. Acquire Lock & Advance Time| Mut
    Mut -->|2. Update Models| State
    Mut -->|3. Reallocate Power| Alloc
    Alloc -->|4. Iterative Proportional Split| State
    
    SC -->|5. Broadcast New State| SSE
    SSE -->|Real-time UI Updates| Client
```

#### Layer Breakdown:

1. **Domain Layer (`internal/domain`)**
   - **Responsibility:** Pure business logic. Completely unaware of concurrency (no mutexes), HTTP requests, or external databases.
   - **Key Components:**
     - `Station`, `EVSE`, `Connector`, `BESS`, `Session`: The core data models representing the physical hardware and active charging sessions.
     - `allocation.go`: Contains the `Allocate()` function and the `iterativeProportionalSplit` algorithm. It gathers the active power demands from all connected EVs, factors in the `GridLimit` and BESS capabilities, and computes exactly how much power each `Connector` receives. It ensures no EVSE or vehicle exceeds its physical constraints and redistributes excess power intelligently.

2. **Service Layer (`internal/service`)**
   - **Responsibility:** Thread-safe orchestration and state management.
   - **Key Components:**
     - `SiteController`: The central brain of the application. It holds the root `domain.Station` model and a `sync.RWMutex` to prevent race conditions.
     - **Mutations:** Functions like `ConnectEV()`, `DisconnectEV()`, and `Tick()`. They lock the state, apply the changes to the domain models, call the domain's reallocation algorithm to recalculate power distributions, and then fire a broadcast event.
     - **Time Simulation:** Manages the deterministic `lastTimestamp` with strict monotonic enforcement (preventing "time travel" via API). When `Tick()` is called, it simulates the passage of time (capped at 10-minute intervals to protect physics calculations), fills up the EV batteries based on their allocated power, and automatically disconnects vehicles that reach 100% SoC.

3. **API Layer (`internal/api`)**
   - **Responsibility:** Translating HTTP requests into Service Layer commands and formatting the output.
   - **Key Components:**
     - `Server` & `Handlers`: Handles parsing JSON requests, validating payload integrity (DTO validation), returning HTTP status codes, and serving the Swagger UI and Web Dashboard. The `/config` endpoint also allows for live hot-swapping of the entire station hardware without downtime.
     - **SSE Streaming**: Maintains an open connection (`http.Flusher`) to connected web clients. When the `SiteController` broadcasts a state change, the API layer immediately flushes the new JSON representation to the browser for true real-time updates.

### Deep Dive: Core Algorithms & Sequence

To understand the core logic of the simulation, we must look at how time and power are processed. The system relies on two main "hard functions": `Tick()` (the time engine) and `Allocate()` (the power distributor).

#### 1. The Time Engine: `SiteController.Tick(duration)`
Because the simulation does not use real-time `time.Now()`, the station's state only advances when `Tick()` is called. This is the heartbeat of the system.

```mermaid
flowchart LR
    API[API Request] --> Lock[1. Lock Station State]
    Lock --> Time[2. Advance Time]
    Time --> Battery[3. Fill EV Batteries]
    Battery --> CheckFull{Is any EV at 100%?}
    
    CheckFull -- Yes --> Disconnect[4. Auto-Disconnect EV]
    Disconnect --> Reallocate
    CheckFull -- No --> Reallocate[5. Reallocate Power]
    
    Reallocate --> SSE[6. Broadcast New State via SSE]
```

**Step-by-step breakdown:**
1. **Lock State:** Pause any other incoming requests to prevent data conflicts.
2. **Advance Time:** Move the simulation clock forward (e.g., +1 minute).
3. **Fill Batteries:** Add energy (kWh) to every connected car based on how much power they were allocated in the previous minute.
4. **Auto-Disconnect:** If a car reaches 100% battery, unplug it automatically.
5. **Reallocate Power:** Because cars might have left, or their charging curves changed, we run the Allocation algorithm to perfectly balance the power again.
6. **Broadcast:** Send the brand new state instantly to the web dashboard so users see the update in real-time.

#### 2. The Power Distributor: `domain.Allocate(station)`
The hardest algorithmic challenge is distributing a limited power budget fairly when hardware constraints (like a 300kW cable limit) cap what an individual vehicle can receive. 

Instead of a simple division (which wastes power when a vehicle hits its cap), the system uses an **Iterative Proportional Split with Limits** (`iterativeProportionalSplit`).

```mermaid
flowchart LR
    Start[Start Allocation] --> Gather[1. gatherDemands\nFind all active sessions requesting power]
    Gather --> Budget[2. computeAvailablePower\nBudget = Grid Limit + BESS Discharge]
    
    Budget --> SplitSite[3. computeSiteShares\nDistribute Budget to EVSEs]
    SplitSite --> Iter1{Is an EVSE capped\nby MaxPower?}
    
    Iter1 -- Yes --> Cap1[Cap that EVSE.\nThrow excess power back into Budget]
    Cap1 --> SplitSite
    
    Iter1 -- No --> SplitConn[4. computeConnectorAllocations\nDistribute EVSE Share to its Connectors]
    
    SplitConn --> Iter2{Is a Connector capped\nby EV limits?}
    
    Iter2 -- Yes --> Cap2[Cap that Connector.\nThrow excess power back into EVSE Share]
    Cap2 --> SplitConn
    
    Iter2 -- No --> Done[5. Return Final Allocations]
```

**How `iterativeProportionalSplit` Works (The Loop):**
1. **Calculate Fair Share:** Divide the remaining budget proportionally based on each participant's requested demand.
2. **Apply Constraints:** Check if any participant's fair share exceeds their physical limit.
3. **Cap & Harvest:** If they exceed the limit, lock them at their maximum limit, subtract that from the budget, and remove them from the active pool.
4. **Redistribute:** Take the excess power they couldn't use and loop back to step 1, redistributing the leftover budget among the *remaining* participants.
5. **Terminate:** The loop ends when no participant hits a limit, ensuring 100% of available power is utilized without violating hardware constraints.

## Design Constraints
- **Pure logic**: No external DB or real-time `time.Now()` is used in the simulation logic.
- **Strict Validation**: All API DTOs and configuration payloads are validated before reaching the Service layer to prevent panics on malformed data.
- **Monotonic Time**: Time advancement is strictly monotonic, preventing backwards time travel via API requests.
- **Tick Cap**: Time leaps are capped at 10 minutes max to prevent mathematical bypasses of hardware constraints (e.g., BESS operational floors).
- **Concurrency**: Lock-free domain layer with concurrency handled at the orchestration (`SiteController`) layer.

## Test Scenarios

The system is validated by a suite of 21 automated tests covering the core domain logic, BESS physics, and API integration. These scenarios were specifically chosen to validate complex proportional math, hardware limits, and concurrent edge cases.

### 1. Power Allocation (Core Algorithm)
*   **`T01_SingleEV`**: Validates the baseline: a single car receives all requested power up to its hardware limits.
*   **`T02_TwoEVs_UnderLimit`**: Validates that when total demand is lower than both the global Grid Limit AND local EVSE maximum limits, multiple EVs receive exactly 100% of their requested power without any throttling.
*   **`T03_TwoEVs_OverLimit`**: Validates the proportional split algorithm when the grid budget is exhausted.
*   **`T04_EVDisconnects`**: Validates that when a vehicle disconnects, its allocated power is instantly released back to the grid budget.
*   **`T05_EVUpdatesPower`**: Validates that when a vehicle updates its charging curve (e.g., requests less power), the freed capacity is instantly re-harvested and distributed to other vehicles.
*   **`T06_EVSESharing`**: Validates the two-level hierarchy logic. If two EVs are on the *same* EVSE, they must fairly split the EVSE's local maximum capacity (e.g., 300kW shared as 150kW each), even if the global Grid Limit has plenty of spare power available overall.
*   **`T07_VehicleLimit`**: **(Edge Case)** Validates the core algorithm's iterative redistribution loop. If a vehicle asks for very little power (e.g., 10kW), but its mathematical proportional share was higher (e.g., 100kW), the algorithm must cap that vehicle at 10kW and intelligently "harvest" the remaining 90kW to redistribute to other power-hungry vehicles on the station.

### 2. BESS Integration (Battery Physics)
*   **`T08_DischargeBoost`**: Validates that the BESS successfully supplements the grid limit during peak demand.
*   **`T09_FloorNoDischarge`**: **(Edge Case)** Validates that a BESS starting at exactly 10% SoC will strictly refuse to discharge, protecting battery health.
*   **`T10_ChargeWithSpare`**: Validates that the BESS actively recharges when total station demand is lower than the grid limit.
*   **`T11_DrainOverTime`**: Validates the arithmetic of energy loss over multi-minute ticks.
*   **`T12_HitsFloorStopsDischarging`**: Validates the mathematical precision of BESS energy drain over time. If a tick duration (e.g., 5 minutes) would theoretically drain the BESS below its strict 10% safety floor, the system must precisely calculate the exact moment it hits 10% and instantly halt discharging, ensuring the floor is never breached even by a fraction of a percent.
*   **`T13_FormatSoC`**: Validates that the state of charge is strictly formatted to exactly 2 decimal places to prevent floating-point precision display issues.

### 3. Time Simulation & State Machine
*   **`T14_TickAdvancesSoC`**: Validates that time progression correctly converts allocated kW power into kWh energy added to the vehicle battery.
*   **`T15_AutoDisconnectAt100`**: Validates the state machine's ability to sever sessions natively when an EV reaches 100% SoC.
*   **`T16_FullChargeCycle`**: Tests an entire end-to-end charge from 10% to 100% across multiple ticks, ensuring mathematically precise energy accumulation without floating-point drift.
*   **`T17_SimultaneousBESSandEV`**: Validates the complex interplay where a BESS is discharging to support an EV, but stops exactly when the EV reaches 100% and auto-disconnects in the exact same tick.

### 4. Integration & Edge Cases
*   **`T18_EndToEnd`**: A full lifecycle integration test. It simulates a sequence of API calls over time: a car connects, the BESS discharges to boost it, the simulation ticks forward, the car updates its charging curve (power request drops), and finally, the car reaches 100% SoC and is safely and automatically disconnected by the Orchestrator.
*   **`T19_EdgeCase_SimultaneousHit`**: **(Concurrency)** Fires 100 concurrent HTTP requests (Connect/Disconnect/PowerUpdate) against the Orchestrator to validate `sync.RWMutex` lock safety and prevent race conditions.
*   **`T20_Performance`**: Benchmarks the allocation algorithm to prove sub-millisecond execution times for real-time responsiveness (<1s latency requirement).

### 5. Configuration & Initialization
*   **`TestLoadStation`**: Validates the JSON parsing, strict structural integrity checks (DTO validation), and correct hierarchical mapping of physical infrastructure (EVSEs and Connectors) into the core domain models.
