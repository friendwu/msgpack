all:
	go test github.com/friendwu/msgpack -cpu=1
	go test github.com/friendwu/msgpack -cpu=2
	go test github.com/friendwu/msgpack -short -race
