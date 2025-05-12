Potential issues:
4. Dashboards are still broken
5. How can the lessons from looking at search effort and result cardinalities be put into play?
    1. Search effort - Modeling input as a trie as done in a previous Racket implementation is most natural
    2. Avoiding result explosions by putting off "free moves" would be helpful
6. OTEL is telling an interesting story - worker is creating a lot of spans.
7. Strip out commas and downcase text
8. Probably split the queue depth panels in grafana and have it display current value
9. Repository says no work items in queue but there are work items in the queue.
10. Consider a better empty queue notification
11. Make sure remediation replays are checking for nonredundancy and relevance
12. Implement streaming dedup in  the feeder