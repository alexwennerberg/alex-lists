"""Add patchset_tool table

Revision ID: 98bb95b72e3a
Revises: 2b20dca45cbb
Create Date: 2020-07-13 14:20:48.315630

"""

# revision identifiers, used by Alembic.
revision = '98bb95b72e3a'
down_revision = '2b20dca45cbb'

from alembic import op
from enum import Enum
import sqlalchemy as sa
import sqlalchemy_utils as sau


class ToolIcon(Enum):
    pending = "pending"
    waiting = "waiting"
    success = "success"
    failed = "failed"
    cancelled = "cancelled"


def upgrade():
    op.create_table('patchset_tool',
        sa.Column('id', sa.Integer, primary_key=True),
        sa.Column('created', sa.DateTime, nullable=False),
        sa.Column('updated', sa.DateTime, nullable=False),
        sa.Column('patchset_id', sa.Integer, sa.ForeignKey('patchset.id', ondelete='CASCADE')),
        sa.Column('icon', sau.ChoiceType(ToolIcon, impl=sa.String()), nullable=False, server_default="pending"),
        sa.Column('details', sa.Unicode, nullable=False),
        sa.Column('key', sa.Unicode(128), nullable=False, index=True))


def downgrade():
    op.drop_table('patchset_tool')
