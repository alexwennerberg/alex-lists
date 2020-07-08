"""Add CASCADEs for list deletion

Revision ID: 79140e9fc031
Revises: 1db0bbd80c48
Create Date: 2020-07-08 11:35:58.476005

"""

# revision identifiers, used by Alembic.
revision = '79140e9fc031'
down_revision = '1db0bbd80c48'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.drop_constraint(
            constraint_name="subscription_list_id_fkey",
            table_name="subscription",
            type_="foreignkey")
    op.create_foreign_key(
            constraint_name="subscription_list_id_fkey",
            source_table="subscription",
            referent_table="list",
            local_cols=["list_id"],
            remote_cols=["id"],
            ondelete="CASCADE")
    op.drop_constraint(
            constraint_name="email_list_id_fkey",
            table_name="email",
            type_="foreignkey")
    op.create_foreign_key(
            constraint_name="email_list_id_fkey",
            source_table="email",
            referent_table="list",
            local_cols=["list_id"],
            remote_cols=["id"],
            ondelete="CASCADE")
    op.drop_constraint(
            constraint_name="email_patchset_id_fkey",
            table_name="email",
            type_="foreignkey")
    op.create_foreign_key(
            constraint_name="email_patchset_id_fkey",
            source_table="email",
            referent_table="patchset",
            local_cols=["patchset_id"],
            remote_cols=["id"],
            ondelete="CASCADE")
    op.drop_constraint(
            constraint_name="patchset_list_id_fkey",
            table_name="patchset",
            type_="foreignkey")
    op.create_foreign_key(
            constraint_name="patchset_list_id_fkey",
            source_table="patchset",
            referent_table="list",
            local_cols=["list_id"],
            remote_cols=["id"],
            ondelete="CASCADE")
    op.drop_constraint(
            constraint_name="list_webhook_delivery_subscription_id_fkey",
            table_name="list_webhook_delivery",
            type_="foreignkey")
    op.create_foreign_key(
            constraint_name="list_webhook_delivery_subscription_id_fkey",
            source_table="list_webhook_delivery",
            referent_table="list_webhook_subscription",
            local_cols=["subscription_id"],
            remote_cols=["id"],
            ondelete="CASCADE")
    op.drop_constraint(
            constraint_name="list_webhook_subscription_list_id_fkey",
            table_name="list_webhook_subscription",
            type_="foreignkey")
    op.create_foreign_key(
            constraint_name="list_webhook_subscription_list_id_fkey",
            source_table="list_webhook_subscription",
            referent_table="list",
            local_cols=["list_id"],
            remote_cols=["id"],
            ondelete="CASCADE")
    op.alter_column("patchset", "list_id", nullable=False)
    op.alter_column("email", "list_id", nullable=False)


def downgrade():
    op.drop_constraint(
            constraint_name="subscription_list_id_fkey",
            table_name="subscription",
            type_="foreignkey")
    op.create_foreign_key(
            constraint_name="subscription_list_id_fkey",
            source_table="subscription",
            referent_table="list",
            local_cols=["list_id"],
            remote_cols=["id"])
    op.drop_constraint(
            constraint_name="email_list_id_fkey",
            table_name="email",
            type_="foreignkey")
    op.create_foreign_key(
            constraint_name="email_list_id_fkey",
            source_table="email",
            referent_table="list",
            local_cols=["list_id"],
            remote_cols=["id"])
    op.drop_constraint(
            constraint_name="email_patchset_id_fkey",
            table_name="email",
            type_="foreignkey")
    op.create_foreign_key(
            constraint_name="email_patchset_id_fkey",
            source_table="email",
            referent_table="patchset",
            local_cols=["patchset_id"],
            remote_cols=["id"])
    op.drop_constraint(
            constraint_name="patchset_list_id_fkey",
            table_name="patchset",
            type_="foreignkey")
    op.create_foreign_key(
            constraint_name="patchset_list_id_fkey",
            source_table="patchset",
            referent_table="list",
            local_cols=["list_id"],
            remote_cols=["id"])
    op.drop_constraint(
            constraint_name="list_webhook_delivery_subscription_id_fkey",
            table_name="list_webhook_delivery",
            type_="foreignkey")
    op.create_foreign_key(
            constraint_name="list_webhook_delivery_subscription_id_fkey",
            source_table="list_webhook_delivery",
            referent_table="list_webhook_subscription",
            local_cols=["subscription_id"],
            remote_cols=["id"])
    op.drop_constraint(
            constraint_name="list_webhook_subscription_list_id_fkey",
            table_name="list_webhook_subscription",
            type_="foreignkey")
    op.create_foreign_key(
            constraint_name="list_webhook_subscription_list_id_fkey",
            source_table="list_webhook_subscription",
            referent_table="list",
            local_cols=["list_id"],
            remote_cols=["id"])
    op.alter_column("patchset", "list_id", nullable=True)
    op.alter_column("email", "list_id", nullable=True)
