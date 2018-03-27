# Cache Server
The range cache server is a concurrent, duplicate-supressing, non-blocking cache server.
It supports range caching. If requests for ranges 0-100, 50-75, and 75-100, then the
range cache will get 50-75 and 75-100 from 0-100.
The cache server checks the source url supports HTTP range requests.
It caches the responses and takes account for overlapping.
Assumes the range cache is a 64 MB cache.


To run the server:
```shell
go run cacheserver.go
```

The URL works with query parameters. 
- url maps to the source URL
- start maps to the start range byte 
- end maps to an optional end range byte.

To see this in action, we will request the first 6.4 MB of a video. Then repeat the request to see the results from the cache server. 
```shell
curl 'http://127.0.0.1:8080/?url=http://storage.googleapis.com/vimeo-test/work-at-vimeo.mp4&start=0&end=6400000' --output out.mp4
```


To run the fake test suite:
```shell
go test -run="Fake" -v -race
cd rangecache/
go test -v -race 
```


Todo
CacheServer should suppress duplicate work when its overlapping work.
Example, 0-100, then supress 50-75 and 75-100.
