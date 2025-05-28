import re
import sqlalchemy as sa
import sqlalchemy_utils as sau
from enum import Enum
from srht.flagtype import FlagType
from srht.database import Base
from listssrht.types.listaccess import ListAccess

class Visibility(Enum):
    PUBLIC = 'PUBLIC'
    UNLISTED = 'UNLISTED'
    PRIVATE = 'PRIVATE'

class List(Base):
    __tablename__ = 'list'
    __table_args__ = sa.UniqueConstraint('owner_id', 'name',
                name="uq_list_owner_id_name"),

    id = sa.Column(sa.Integer, primary_key=True)
    created = sa.Column(sa.DateTime, nullable=False)
    updated = sa.Column(sa.DateTime, nullable=False)
    name = sa.Column(sa.String(128), nullable=False)
    description = sa.Column(sa.Unicode(2048))
    visibility = sa.Column(sau.ChoiceType(Visibility), nullable=False)
    import_in_progress = sa.Column(
            sa.Boolean, nullable=False, server_default='f')

    default_access = sa.Column(FlagType(ListAccess),
            nullable=False, server_default=str(ListAccess.normal.value))

    permit_mimetypes = sa.Column(sa.Unicode, nullable=False,
            server_default="text/*,application/pgp-signature,application/pgp-keys")
    reject_mimetypes = sa.Column(sa.Unicode, nullable=False, server_default="")

    owner_id = sa.Column(sa.Integer, sa.ForeignKey('user.id'), nullable=False)
    owner = sa.orm.relationship('User', backref=sa.orm.backref('lists'))

    mirror_id = sa.Column(sa.Integer, sa.ForeignKey('mirror.id'))
    mirror = sa.orm.relationship("Mirror", uselist=False, back_populates="list")

    last_activity = sa.Column(sa.DateTime, nullable=True)

    def __init__(self, owner, valid):
        self.owner = owner
        self.owner_id = owner.id
        self.name = valid.require("name", friendly_name="Name")
        self.description = valid.optional("description")
        if not valid.ok:
            return
        valid.expect(re.match(r'^[A-Za-z0-9._-]+$', self.name),
                "Name must match [A-Za-z0-9._-]+", field="name")
        valid.expect(self.name not in [".", ".."],
                "Name cannot be '.' or '..'", field="name")
        valid.expect(self.name not in [".git", ".hg"],
                "Name must not be '.git' or '.hg'", field="name")
        existing = (List.query
                .filter(List.owner_id == owner.id)
                .filter(List.name.ilike(self.name.replace('_', '\\_')))
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
                    "nonsubscriber": permissions(self.default_access),
                    "subscriber": permissions(self.default_access),
                    "account": permissions(self.default_access),
                },
            } if not short else {})
        }
