"""6_fixmasurements

Revision ID: f16c6875f138
Revises: 4b9e4dc2bc99
Create Date: 2017-05-11 14:09:41.293021

"""
from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = 'f16c6875f138'
down_revision = '4b9e4dc2bc99'
branch_labels = None
depends_on = None


def upgrade():
        op.execute('''
            ALTER TABLE measurement DROP COLUMN type;
        ''')
        op.execute('''
            ALTER TABLE instrument ADD COLUMN key varchar(255);
        ''')


def downgrade():
    pass
