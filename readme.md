# PiTilt API


## run

```go run *.go```


## Enable db

```cd db```

```vagrant up```


## Alembic

```cd db```

```vagrant up```

```vagrant ssh```

```cd /vagrant/```

```source /opt/alembic/venv/bin/activate```

```export DATABASE_URI=databaseuri```

```alembic revision -m "N-message"```

```alembic upgrade head```
