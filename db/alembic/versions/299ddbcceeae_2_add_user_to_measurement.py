"""2-add-user-to-measurement

Revision ID: 299ddbcceeae
Revises: 43a8acfdd2ee
Create Date: 2017-03-15 00:13:41.789036

"""
from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = '299ddbcceeae'
down_revision = '43a8acfdd2ee'
branch_labels = None
depends_on = None


def upgrade():
    op.execute('''
        CREATE TABLE login (
            id varchar(255) PRIMARY KEY,
            first_name varchar(255),
            last_name varchar(255),
            email varchar(255),
            key varchar(255)
        );
    ''')
    op.execute('''
        INSERT INTO login (id, first_name, last_name, email, key) VALUES ('1', 'Atle', 'Sveen', 'atle@frenviksveen.net', 'YOUR_KEY')
    ''')
    op.execute('''
        ALTER TABLE measurement ADD COLUMN login varchar(255) REFERENCES login (id);
    ''')


def downgrade():
    op.execute('''
        ALTER TABLE measurement DROP constraint measurement_user_fkey
    ''')
    op.execute('''
        ALTER TABLE measurement DROP column login;
    ''')
    op.execute('''
        DROP TABLE login
    ''')