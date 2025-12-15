# Architecture Documentation

## Design Principles

1. **Stealth First**: Every interaction is designed to mimic human behavior. We avoid direct API calls and rely solely on browser automation (Rod) with heavy augmentation for meaningful noise (mouse movements, delays).
2. **Modularity**: Components are decoupled. `search` knows nothing about `connection`. `bot` acts as the orchestrator.
3. **Persistence**: SQLite is used for robust state management. Usage of `modernc.org/sqlite` ensures CGO-free portability.

## Components

### Browser & Stealth
- **Manager**: Handles browser lifecycle. Checks for `headless` config.
- **Stealth Primitives**: `stealth` package provides reusable behaviors. `Mouse` uses math-based curves. `Typer` introduces human error.

### Authentication
- Uses AES-256 encryption for cookie storage to secure session data on disk.
- Fallback to manual login if session restore fails.

### Storage
- SQLite schema allows for relational tracking (Profiles -> Requests -> Messages).
- Bloom Filter in `search` provides O(1) duplicate checks to save DB hits.

## Data Flow

1. **Search**: Scrapes profiles -> Parsed -> Deduped (Bloom) -> DB (Profiles).
2. **Connect**: DB (Uncontacted Profiles) -> Rate Limiter Check -> Stealth Nav/Click -> DB (Requests).
3. **Message**: Detect New Connections (Page Scrape) -> DB (Match ID) -> Render Template -> Send -> DB (Messages).
