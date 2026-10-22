package daomock

//go:generate go run go.uber.org/mock/mockgen@v0.4.0 -source=../../dao/task.go -destination=mock_dao.go -package=$GOPACKAGE
