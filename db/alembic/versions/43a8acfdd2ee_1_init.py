"""1-init

Revision ID: 43a8acfdd2ee
Revises: 
Create Date: 2017-03-14 21:14:23.290347

"""
from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = '43a8acfdd2ee'
down_revision = None
branch_labels = None
depends_on = None


def upgrade():
    op.execute('''
    CREATE TABLE measurement (
        id serial PRIMARY KEY,
        color varchar(255),
        gravity int,
        temperature double precision,
        ts timestamp with time zone
    );
    ''')


def downgrade():
    op.execute('''
        DROP TABLE measurement
    ''')
