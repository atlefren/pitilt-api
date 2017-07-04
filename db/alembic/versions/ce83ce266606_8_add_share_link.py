"""8-add_share_link

Revision ID: ce83ce266606
Revises: 0a9c03afe9d5
Create Date: 2017-07-04 19:09:13.568901

"""
from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = 'ce83ce266606'
down_revision = '0a9c03afe9d5'
branch_labels = None
depends_on = None


def upgrade():
    op.execute('''
    CREATE TABLE sharelink (
        plot_id integer UNIQUE NOT NULL REFERENCES plot ON DELETE CASCADE,
        uuid varchar(255) UNIQUE NOT NULL
    );
    ''')



def downgrade():
    op.execute('''
    DROP TABLE sharelink
    ''')
