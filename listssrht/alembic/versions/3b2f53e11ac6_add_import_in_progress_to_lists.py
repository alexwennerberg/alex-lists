"""Add import_in_progress to lists

Revision ID: 3b2f53e11ac6
Revises: f5ec39205a69
Create Date: 2019-04-08 15:43:58.131507

"""

# revision identifiers, used by Alembic.
revision = '3b2f53e11ac6'
down_revision = 'f5ec39205a69'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.add_column('list', sa.Column('import_in_progress',
        sa.Boolean, nullable=False, server_default='f'))


def downgrade():
    op.drop_column('list', 'import_in_progress')
