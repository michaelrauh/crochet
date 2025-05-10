Potential issues:
1. Will the remediations from the first ingestion get played on itself?
2. Why is ingest slow?
3. Will the DB queue ever catch up? Does it need to?
4. Dashboards are still broken
5. How can the lessons from looking at search effort and result cardinalities be put into play?
    1. Search effort - Modeling input as a trie as done in a previous Racket implementation is most natural
    2. Avoiding result explosions by putting off "free moves" would be helpful
6. OTEL is telling an interesting story - worker is creating a lot of spans.