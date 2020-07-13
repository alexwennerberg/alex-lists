"""Add submitter to patchset

Revision ID: 2b20dca45cbb
Revises: 79140e9fc031
Create Date: 2020-07-13 12:45:50.230226

"""

# revision identifiers, used by Alembic.
revision = '2b20dca45cbb'
down_revision = '79140e9fc031'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.add_column('patchset', sa.Column('submitter', sa.Unicode))
    op.add_column('patchset', sa.Column('message_id', sa.Unicode))


def downgrade():
    op.drop_column('patchset', 'submitter')
    op.drop_column('patchset', 'message_id')
