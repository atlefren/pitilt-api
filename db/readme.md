
## Setup

1. install vagrant
2. run vagrant up


## Alembic

cd /opt/alembic/

source /opt/alembic/venv/scripts/activate
alembic revision -m "init database"
export DATABASE_URI=postgres://tilt:password@localhost:5432/tilt
alembic upgrade head


