#PiTilt API


##run
```go run *.go```


##enable db

```cd db```
```vagrant up```


##alembic
```cd db```
```vagrant up```
```vagrant ssh```
```source  ~/venv/scripts/activate```
```export DATABASE_URI=databaseuri```
```cd /vagrant```
```alembic revision -m "N-message"```
```alembic upgrade head```
