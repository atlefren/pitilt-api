"""7-fix-login-name

Revision ID: 0a9c03afe9d5
Revises: 71213454a09a
Create Date: 2017-05-16 06:45:55.566096

"""
from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = '0a9c03afe9d5'
down_revision = '71213454a09a'
branch_labels = None
depends_on = None


def upgrade():
    # No need to merge the column data since last_name
    # is always empty.
    op.execute('''
        ALTER TABLE login RENAME COLUMN first_name TO name;
        ALTER TABLE login DROP COLUMN last_name;
    ''')


def downgrade():
    op.execute('''
        ALTER TABLE login RENAME COLUMN name TO first_name;
        ALTER TABLE login ADD COLUMN last_name varchar(255);
    ''')
