r"""Draws the AWS architecture diagram, docs/architecture.png.

Needs Python 3, Graphviz (dot on PATH) and the diagrams package, which
bundles the official AWS Architecture Icons:

    python -m venv <venv>
    <venv>/bin/pip install diagrams==0.25.1   # <venv>\Scripts\pip on Windows
    <venv>/bin/python tools/architecture_diagram.py [output] [dpi]

The output defaults to docs/architecture (".png" is added) and the dpi to 90,
which makes the image about 1900 pixels wide. On Windows, put the venv at a
short path: some icon paths are long, and pip fails without long path support.
"""

import os
import sys

from diagrams import Cluster, Diagram, Edge
from diagrams.aws.compute import Lambda
from diagrams.aws.database import Dynamodb
from diagrams.aws.engagement import SimpleEmailServiceSes
from diagrams.aws.general import User, Users
from diagrams.aws.integration import Eventbridge, SimpleNotificationServiceSns, SimpleQueueServiceSqs
from diagrams.aws.management import Cloudwatch, CloudwatchLogs, SystemsManagerParameterStore
from diagrams.aws.network import APIGateway, Route53
from diagrams.aws.storage import S3

repo = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
out = sys.argv[1] if len(sys.argv) > 1 else os.path.join(repo, "docs", "architecture")
dpi = sys.argv[2] if len(sys.argv) > 2 else "90"

GREY, PINK, GREEN, RED = "#545B64", "#C7166F", "#1E8900", "#D13212"

graph_attr = {
    "fontsize": "26",
    "fontname": "Helvetica",
    "labelloc": "t",
    "pad": "0.3",
    "nodesep": "0.35",
    "ranksep": "1.3",
    "dpi": dpi,
    "splines": "spline",
    "newrank": "true",
}
node_attr = {"fontsize": "13", "fontname": "Helvetica"}
edge_attr = {"fontsize": "12", "fontname": "Helvetica", "color": GREY, "penwidth": "1.3"}


def edge(text="", color=GREY, style="solid", **attrs):
    return Edge(label=text, color=color, fontcolor=color, style=style, **attrs)


with Diagram(
    "Dead Man's Handle",
    filename=out,
    outformat="png",
    show=False,
    direction="LR",
    graph_attr=graph_attr,
    node_attr=node_attr,
    edge_attr=edge_attr,
):
    owner = User("Owner")

    with Cluster("AWS account (one region)"):
        with Cluster("Check-in"):
            api = APIGateway("HTTP API\nPOST /checkin")
            http_fn = Lambda("http")

        with Cluster("Daily run"):
            schedule = Eventbridge("Schedule rule\n02:00 UTC")
            scheduled_fn = Lambda("scheduled")

        with Cluster("Document changes"):
            upload_rule = Eventbridge("Rule: document\nuploaded")
            docwatch_fn = Lambda("docwatch")

        with Cluster("Config and state"):
            config = SystemsManagerParameterStore("Config (SecureString)\nread by every Lambda")
            state = Dynamodb("State item")

        bucket = S3("Document bucket")

        with Cluster("Email"):
            dns = Route53("Route 53\nDKIM, DMARC")
            ses = SimpleEmailServiceSes("SES")

        with Cluster("Monitoring"):
            dlq = SimpleQueueServiceSqs("Dead-letter\nqueue")
            alarms = Cloudwatch("CloudWatch alarms\nerrors, not run,\nDLQ, API 4xx")
            topic = SimpleNotificationServiceSns("Alarm topic")
            logs = CloudwatchLogs("Logs\n(14 days)")

    inbox = User("Owner's inbox")
    recipients = Users("Recipients")

    # Check-in
    owner >> edge("check in\n(x-api-key)", PINK) >> api >> http_fn
    http_fn >> edge("read config\n(cached 5 min)") >> config
    http_fn >> edge("reset timeout") >> state

    # Daily run
    schedule >> scheduled_fn
    scheduled_fn >> edge("read, record sends") >> state
    scheduled_fn >> edge("HeadObject, GetObject") >> bucket

    # Document changes: the bucket's events come back to the rule
    bucket >> edge("Object Created", PINK, constraint="false") >> upload_rule
    upload_rule >> docwatch_fn
    docwatch_fn >> edge("compare ETag") >> state
    docwatch_fn >> edge("HeadObject") >> bucket

    # Email
    scheduled_fn >> edge("warnings, document") >> ses
    docwatch_fn >> edge("change notice") >> ses
    dns >> edge("signs", style="dashed") >> ses
    ses >> edge("document", GREEN) >> recipients
    ses >> edge("warnings, notices", GREEN) >> inbox

    # Monitoring
    [schedule, upload_rule] >> edge("failed delivery", RED, "dashed") >> dlq
    dlq >> alarms >> topic >> edge("alarm email", RED) >> inbox

    # Layout only: SES in the column after the data stores, logs bottom left
    bucket >> Edge(style="invis") >> ses
    logs >> Edge(style="invis") >> dlq
