import re
import sqlalchemy as sa
from srht.flagtype import FlagType
from srht.database import Base
from listssrht.types.listaccess import ListAccess

class List(Base):
    __tablename__ = 'list'
    id = sa.Column(sa.Integer, primary_key=True)
    created = sa.Column(sa.DateTime, nullable=False)
    updated = sa.Column(sa.DateTime, nullable=False)
    name = sa.Column(sa.String(128), nullable=False)
    description = sa.Column(sa.Unicode(2048))
    import_in_progress = sa.Column(
            sa.Boolean, nullable=False, server_default='f')

    nonsubscriber_permissions = sa.Column(FlagType(ListAccess),
            nullable=False, server_default=str(ListAccess.normal.value))
    """
    Permissions granted to users who are not subscribed or logged in.
    """

    subscriber_permissions = sa.Column(FlagType(ListAccess),
            nullable=False, server_default=str(ListAccess.normal.value))
    """
    Permissions granted to users who are subscribed to the list.
    """

    account_permissions = sa.Column(FlagType(ListAccess),
            nullable=False, server_default=str(ListAccess.normal.value))
    """
    Permissions granted to holders of sr.ht accounts.
    """

    permit_mimetypes = sa.Column(sa.Unicode, nullable=False,
            server_default="text/*,application/pgp-signature,application/pgp-keys")
    reject_mimetypes = sa.Column(sa.Unicode, nullable=False, server_default="")

    owner_id = sa.Column(sa.Integer, sa.ForeignKey('user.id'), nullable=False)
    owner = sa.orm.relationship('User', backref=sa.orm.backref('lists'))

    mirror_id = sa.Column(sa.Integer, sa.ForeignKey('mirror.id'))
    mirror = sa.orm.relationship("Mirror", uselist=False, back_populates="list")

    def __init__(self, owner, valid):
        self.owner = owner
        self.owner_id = owner.id
        self.name = valid.require("name", friendly_name="Name")
        self.description = valid.optional("description")
        if not valid.ok:
            return
        valid.expect(re.match(r'^[a-z_-][a-z0-9._-]*$', self.name),
                "Name must match [a-z_-][a-z0-9._-]*", field="name")
        valid.expect(self.name not in [".", ".."],
                "Name cannot be '.' or '..'", field="name")
        existing = (List.query
                .filter(List.owner_id == owner.id)
                .filter(List.name.ilike(self.name))
                .first())
        valid.expect(not existing,
                "This name is already in use.", field="name")
        valid.expect(not self.description or len(self.description) < 2048,
                "Description must be between fewer than 2048 characters.",
                field="description")

    def update(self, valid):
        self.description = valid.optional("description",
                default=self.description)
        # TODO: Update permissions

    def __repr__(self):
        return '<List {} {}>'.format(self.id, self.name)

    def to_dict(self, short=False):
        def permissions(perm):
            return [p.name for p in ListAccess
                    if p in perm and p not in [ListAccess.none, ListAccess.all]]
        return {
            "id": self.id,
            "name": self.name,
            "owner": self.owner.to_dict(short=True),
            **({
                "created": self.created,
                "updated": self.updated,
                "description": self.description,
                "permissions": {
                    "nonsubscriber": permissions(self.nonsubscriber_permissions),
                    "subscriber": permissions(self.subscriber_permissions),
                    "account": permissions(self.account_permissions),
                },
            } if not short else {})
        }
