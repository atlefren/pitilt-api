"""4 cleanup

Revision ID: dde814c146ac
Revises: ab7e839f2b2d
Create Date: 2017-03-30 20:16:38.514170

"""
from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = 'dde814c146ac'
down_revision = 'ab7e839f2b2d'
branch_labels = None
depends_on = None


def upgrade():
    op.execute('''
        DELETE FROM measurement where name is null;
        ALTER TABLE measurement RENAME COLUMN ts TO timestamp;
    ''')


def downgrade():
    op.execute('''
        DELETE FROM measurement where name is null;
        ALTER TABLE measurement RENAME COLUMN timestamp TO ts;
    ''')
