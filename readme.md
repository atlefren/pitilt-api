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
```cd /vagrant/```
```source /opt/alembic/venv/bin/activate```
```export DATABASE_URI=databaseuri```
```alembic revision -m "N-message"```
```alembic upgrade head```
