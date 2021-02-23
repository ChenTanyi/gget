module github.com/chentanyi/gget

go 1.13

replace github.com/chentanyi/gget/downloader => ./downloader

require (
	github.com/chentanyi/go-utils v0.0.0-20200625014635-e02f4ec6c25e
	github.com/sirupsen/logrus v1.8.0
	golang.org/x/net v0.0.0-20210222171744-9060382bd457
)
