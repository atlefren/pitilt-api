"""3 add plot

Revision ID: ab7e839f2b2d
Revises: 299ddbcceeae
Create Date: 2017-03-30 19:57:12.409636

"""
from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = 'ab7e839f2b2d'
down_revision = '299ddbcceeae'
branch_labels = None
depends_on = None


def upgrade():

    op.execute('''
        ALTER TABLE measurement ADD COLUMN name varchar(255);
        ALTER TABLE measurement ADD COLUMN type varchar(255);
        ALTER TABLE measurement ADD COLUMN value double precision;
        --UPDATE TABLE measurement set name = 'tilt_temp', type='temperature_celsius', value=temperature
        insert into measurement (name, type, ts, value, login) SELECT 'tilt_temp', 'temperature_celsius', ts, temperature, login from measurement;
        insert into measurement (name, type, ts, value, login) SELECT 'tilt_gravity', 'gravity', ts, cast(gravity as float), login from measurement;

    ''')

    op.execute('''
        CREATE TABLE plot (
            id serial PRIMARY KEY,
            start_time timestamp,
            end_time timestamp,
            name varchar (255),
            login varchar(255) REFERENCES login (id)
        );
    ''')

    op.execute('''
        CREATE TABLE instrument (
            id serial PRIMARY KEY,
            name varchar(255),
            type varchar(255),
            plot int REFERENCES plot (id)
        );
    ''')


def downgrade():
    op.execute('''
        DROP TABLE plot
    ''')
    op.execute('''
        DROP TABLE instrument
    ''')
