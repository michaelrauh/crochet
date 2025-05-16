1. Split out work items and defer "up" expansions which are unconstrained
1. Implement streaming dedup in the feeder
1. Convert intersect finding to precalculated bitsets and distribute them via files for MMapping on the worker nodes (rather than looping over vocab)
1. Add sharding to the feeder and DB - this involves only one shard managing ingestion and notifying repository when done so that it can issue a done sigil to the other shards