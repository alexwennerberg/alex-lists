"""Add more patch fields to email table

Revision ID: 223cd3247273
Revises: 3b2f53e11ac6
Create Date: 2019-05-07 14:45:00.548825

"""

# revision identifiers, used by Alembic.
revision = '223cd3247273'
down_revision = '3b2f53e11ac6'

from alembic import op
from enum import Enum
import sqlalchemy as sa
import sqlalchemy_utils as sau

class PatchSetStatus(Enum):
    proposed = "proposed"
    needs_revision = "needs_revision"
    approved = "approved"
    rejected = "rejected"
    applied = "applied"

def upgrade():
    op.add_column('email', sa.Column("patch_index", sa.Integer))
    op.add_column('email', sa.Column("patch_count", sa.Integer))
    op.add_column('email', sa.Column("patch_version", sa.Integer))
    op.add_column('email', sa.Column("patch_prefix", sa.Unicode))
    op.add_column('email', sa.Column("patch_subject", sa.Unicode))
    op.add_column('email', sa.Column("superseded_by_id",
        sa.Integer, sa.ForeignKey('email.id')))
    op.create_table('patchset',
        sa.Column("id", sa.Integer, primary_key=True),
        sa.Column("created", sa.DateTime, nullable=False),
        sa.Column("updated", sa.DateTime, nullable=False),
        sa.Column("subject", sa.Unicode(2048), nullable=False),
        sa.Column("prefix", sa.Unicode),
        sa.Column("version", sa.Integer, nullable=False),
        sa.Column("status", sau.ChoiceType(PatchSetStatus, impl=sa.String()),
            nullable=False, server_default="proposed"),
        sa.Column("list_id", sa.Integer, sa.ForeignKey('list.id')),
        sa.Column("cover_letter_id", sa.Integer, sa.ForeignKey('email.id')),
        sa.Column("superseded_by_id", sa.Integer, sa.ForeignKey('patchset.id')))
    op.add_column('email', sa.Column('patchset_id',
        sa.Integer, sa.ForeignKey('patchset.id')))


def downgrade():
    op.drop_column('email', 'patch_index')
    op.drop_column('email', 'patch_count')
    op.drop_column('email', 'patch_version')
    op.drop_column('email', 'patch_prefix')
    op.drop_column('email', 'patch_subject')
    op.drop_column('email', 'superseded_by_id')
    op.drop_column('email', 'patchset_id')
    op.drop_table('patchset')
