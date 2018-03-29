# Cache Server
The range cache server is a concurrent, duplicate-supressing, non-blocking cache server.
It supports range caching. If requests for ranges 0-100, 50-75, and 75-100, then the
range cache will get 50-75 and 75-100 from 0-100.
The cache server checks the source url supports HTTP range requests.
It caches the responses and takes account for overlapping.
Assumes the range cache is a 64 MB cache.

The URL works with query parameters. 
- url maps to the source URL
- start maps to the start range byte 
- end maps to an optional end range byte.

To see this in action, we will request the first 6.4 MB of a video. Then repeat the request to see the results from the cache server. 
```shell
$ curl 'http://127.0.0.1:8080/?url=http://storage.googleapis.com/vimeo-test/work-at-vimeo.mp4&start=0&end=6400000'

## Run
To run the server, open two shells. In one run:
```shell
$ go run cacheserver.go
```
Then request the video by running:
```shell
$ time curl 'http://127.0.0.1:8080/?url=http://storage.googleapis.com/vimeo-test/work-at-vimeo.mp4&start=0&end=6400000'
```

## Example Run
Shell for cacheserver
```shell
$ go run cacheserver.go
cache miss: 3.478388958s, GET: bytes=0-6400000
cache hit: 6.915338ms, GET: bytes=0-6000000
cache hit: 13.667057ms, GET: bytes=0-6400000
```

Shell for curl requests
```shell
$ time curl 'http://127.0.0.1:8080/?url=http://storage.googleapis.com/vimeo-test/work-at-vimeo.mp4&start=0&end=6400000'
real	0m3.629s
$ time curl 'http://127.0.0.1:8080/?url=http://storage.googleapis.com/vimeo-test/work-at-vimeo.mp4&start=0&end=6000000'
real	0m0.237s
$ time curl 'http://127.0.0.1:8080/?url=http://storage.googleapis.com/vimeo-test/work-at-vimeo.mp4&start=0&end=6400000'
real	0m0.114s
```


## Tests
To run the fake test suite:
```shell
$ go test -run="Fake" -v -race
=== RUN   TestSequentialFake
cache miss: 873.336026ms, GET: {0 640}
time: 874.107859ms: GetRange({0 640}), 641 bytes
cache hit: 12.024µs, GET: {0 320}
time: 57.515µs: GetRange({0 320}), 321 bytes
…
--- PASS: TestSequentialFake (4.09s)
=== RUN   TestConcurrentFake
first request: {0 640}
first request: {0 320}
…
repeated request: {0 640}
repeated request: {0 320}
…
867.048257ms: GetRange({0 640}), 641 bytes
429.745656ms: GetRange({0 320}), 321 bytes
…
--- PASS: TestConcurrentFake (1.74s)
PASS
ok  	github.com/zeneli/cacheserver	6.851s

$ cd rangecache/
$ go test -v -race 
=== RUN   TestGet
exact range match
2.44155ms, Add({0 100})
544.678µs, Add({50 75})
528.118µs, Add({75 100})
5.148µs, Get({0 100})
1.119µs, Get({50 75})
3.212µs, Get({75 100})
…
--- PASS: TestGet (0.01s)
PASS
ok  	github.com/zeneli/cacheserver/rangecache	1.027s
```


## Features
Todo: CacheServer should suppress duplicate work when its overlapping work.
Example, 0-100, then supress 50-75 and 75-100.

## Note
These example runs are done on a MacBook Pro 15-inch, Mid 2015 with 2.5GHz Intel Core i7.
