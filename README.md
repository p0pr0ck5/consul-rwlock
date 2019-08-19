# consul-rwlock

Distributed writer-preferred rwlock, powered by Consul.

## Synopsis

`consul-rwlock` provides a distributed read-write lock, using Consul
KV store to maintain the lock mechanisms. Consul KV CAS queries are
used in conjunction with blocking queries to provide atomic adjustment
of the read and writer counters. Writer starvation is avoided by preventing
readers from acquiring a lock when a writer is pending lock acquisition.

Exported functions are similar to those provided by the `RWMutex` made
available from [`sync`](https://golang.org/pkg/sync/), with the exception
that all functions return a possible error passed directly from the Consul
API. This allows for drop-in replacement of existing `RWMutex` code,
though replacement without error handling is strongly discouraged outside of
a playground.

## Usage

See `example/main.go` for example usage.

## License

Copyright 2019 Robert Paprocki.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
