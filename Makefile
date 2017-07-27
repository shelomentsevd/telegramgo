all: 
	go build -i -v -o telegram telegram.go utils.go
clean: 
	@rm telegram
