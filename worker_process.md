```mermaid
sequenceDiagram
    participant Search
    participant Repository

    Search->>Repository: GET /Work
    Repository-->>Search: Reply (200, Work)
    Search->>Search: Log Work Retrieval
    Note right of Search: Log work details including version
    Search->>Search: Check Version
        alt Version Mismatch
            Search->>Repository: GET /Context
            Repository-->>Search: Reply (200, Context)
            Search->>Search: Log Context Retrieval
        end

    Search->>Repository: POST Results (Orthos, Remediations)
    Search->>Search: Log Results Posted
```