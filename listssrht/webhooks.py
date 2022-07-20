import sqlalchemy as sa
from srht.config import cfg
from srht.database import DbSession, db
if not hasattr(db, "session"):
    # Initialize the database if not already configured (for running daemon)
    db = DbSession(cfg("lists.sr.ht", "connection-string"))
    import listssrht.types
    db.init()
from srht.webhook import Event
from srht.webhook.celery import CeleryWebhook, make_worker
from srht.metrics import RedisQueueCollector

webhook_broker = cfg("lists.sr.ht", "webhooks")
worker = make_worker(broker=webhook_broker)
webhook_metrics_collector = RedisQueueCollector(webhook_broker, "srht_webhooks", "Webhook queue length")

class ListWebhook(CeleryWebhook):
    events = [
        Event("post:received", "lists:read"),
        Event("list:update", "lists:read"),
        Event("list:delete", "lists:read"),
        Event("patchset:received", "patches:read"),
        Event("patchset:update", "patches:read"), # TODO: Deliver
    ]

    list_id = sa.Column(sa.Integer, sa.ForeignKey("list.id", ondelete="CASCADE"))
    list = sa.orm.relationship("List")

class UserWebhook(CeleryWebhook):
    events = [
        Event("email:received", "emails:read"),
        Event("list:create", "lists:read"),
    ]
