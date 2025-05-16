```mermaid
sequenceDiagram
    participant Search
    participant Repository
    participant Work Queue 
    participant DB Queue
    Search->>Repository: POST /Results(Orthos, Remediations, Receipt)
    Repository->>DB Queue: Orthos
    Repository->>DB Queue: Remediations
    Repository->>Work Queue: Ack(receipt) 
    Repository->>Search: Reply (200, Context)    
``` 