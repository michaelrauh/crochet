```mermaid
sequenceDiagram
    participant User
    participant Repository
    participant DBQueue as DB Queue
    participant Feeder
    participant DB
    participant WorkQueue as Work Queue
    participant Search 
    participant LRU

    %% 1. User submits new corpora
    User->>Repository: POST /Corpora (Title, Text)
    Repository->>DBQueue: Push(Context)
    Repository->>DBQueue: Push(Version)
    Repository->>DBQueue: Push(Pairs) 
    Repository->>DBQueue: Push(Seed)
    Repository-->>User: Reply (202)

    %% 2. Feeder pops and processes DB queue items
    DBQueue->>Feeder: Pop Batch
    alt Version
        Feeder->>DB: Update Version
        Feeder->>DB: Commit
    else Pair
        Feeder->>DB: Look Up Orthos from Pair
        Feeder->>WorkQueue: Push Ortho
        Feeder->>DBQueue: Push Remediation Delete
    else Context
        Feeder->>DB: Upsert Context
    else Ortho
        Feeder->>DB: Upsert Ortho
        Feeder->>WorkQueue: Push Ortho
    else RemediationDelete
        Feeder->>DB: Delete Remediation
    else Remediation
        Feeder->>DB: Upsert Remediation
    end

    %% 3. Search fetches work
    Search->>Repository: GET /Work
    Repository->>DB: Read(Version)
    Repository->>WorkQueue: Pop
    WorkQueue-->>Repository: (Work, Receipt)
    Repository-->>Search: Reply (200, Version, Work, Receipt)

    alt Version Mismatch
        Search->>Repository: GET /Context
        Repository-->>Search: Reply (200, Context)
    end

    %% 4. Search posts results
    Search->>Repository: POST /Results (Orthos, Remediations, Receipt)
    Repository->>LRU: Diff(Orthos)
    LRU-->>Repository: New Orthos
    Repository->>DBQueue: New Orthos
    Repository->>DBQueue: Remediations
    Repository->>WorkQueue: Ack(Receipt)
    Repository-->>Search: Reply (200, Context)