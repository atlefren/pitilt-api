"""5 cleanup

Revision ID: 4b9e4dc2bc99
Revises: dde814c146ac
Create Date: 2017-03-30 20:19:38.104060

"""
from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = '4b9e4dc2bc99'
down_revision = 'dde814c146ac'
branch_labels = None
depends_on = None


def upgrade():
        op.execute('''
        ALTER TABLE measurement RENAME COLUMN name to key;
        ALTER TABLE measurement DROP COLUMN color;
        ALTER TABLE measurement DROP COLUMN gravity;
        ALTER TABLE measurement DROP COLUMN temperature;
    ''')


def downgrade():
    pass
