"""Add mirror table

Revision ID: 617471810f6f
Revises: 803817bfd568
Create Date: 2019-03-11 20:44:41.864542

"""

# revision identifiers, used by Alembic.
revision = '617471810f6f'
down_revision = '803817bfd568'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.create_table("mirror",
        sa.Column("id", sa.Integer, primary_key=True),
        sa.Column("created", sa.DateTime, nullable=False),
        sa.Column("updated", sa.DateTime, nullable=False),
        sa.Column("configure_attempts", sa.Integer, nullable=False, server_default='0'),
        sa.Column("configured", sa.Boolean, nullable=False, server_default='f'),
        sa.Column("mailer_sender", sa.Unicode),
        sa.Column("list_subscribe", sa.Unicode),
        sa.Column("list_unsubscribe", sa.Unicode),
        sa.Column("list_post", sa.Unicode),
    )
    op.add_column("list",
            sa.Column("mirror_id", sa.Integer, sa.ForeignKey("mirror.id")))


def downgrade():
    op.drop_column("list", "mirror_id")
    op.drop_table("mirror")
