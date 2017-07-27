all: install 
install:
	@ go install
run:
	@ go run main.go
clean: 
	@ rm telegram
