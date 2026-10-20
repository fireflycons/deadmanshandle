r"""Draws the AWS architecture diagram, docs/architecture.svg, which
docs/architecture.html shows.

Needs Python 3, Graphviz (dot on PATH), the diagrams package for the layout,
and AWS's Architecture Icons package for the icons:

    python -m venv <venv>
    <venv>/bin/pip install diagrams==0.25.1   # <venv>\Scripts\pip on Windows
    <venv>/bin/python tools/architecture_diagram.py --icons <dir> [output]

<dir> is the unzipped icon package, from
https://aws.amazon.com/architecture/icons/ (drawn with the 07312026 release:
https://d1.awsstatic.com/onedam/marketing-channels/website/public/shared/architecture-icon-release/Icon-package_07312026.5846e92413caa21490223536cc97f1269e44fa92.zip).
The output defaults to docs/architecture (".svg" is added). The icons are
embedded in the SVG, so it needs no other files. Graphviz (8.0.5, at least)
lays the same diagram out differently from run to run: rerun it until the
layout reads well, and check it in a browser. On Windows, put the venv and
the icons at a short path: some paths are long, and pip fails without long
path support.
"""

import argparse
import base64
import glob
import os
import re
import shutil
import tempfile

import diagrams
from diagrams import Cluster, Diagram, Edge
from diagrams.custom import Custom

repo = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
parser = argparse.ArgumentParser(description="Draw docs/architecture.svg")
parser.add_argument("--icons", required=True, help="the unzipped AWS Architecture Icons package")
parser.add_argument("output", nargs="?", default=os.path.join(repo, "docs", "architecture"))
args = parser.parse_args()

# Each icon used, as a path (glob) in the icon package. Firebase isn't AWS:
# its icon is the one bundled with diagrams.
ICONS = {
    "lambda": "Architecture-Service-Icons_*/Arch_Compute/64/Arch_AWS-Lambda_64.svg",
    "api-gateway": "Architecture-Service-Icons_*/Arch_Networking-Content-Delivery/64/Arch_Amazon-API-Gateway_64.svg",
    "route-53": "Architecture-Service-Icons_*/Arch_Networking-Content-Delivery/64/Arch_Amazon-Route-53_64.svg",
    "s3": "Architecture-Service-Icons_*/Arch_Storage/64/Arch_Amazon-Simple-Storage-Service_64.svg",
    "dynamodb": "Architecture-Service-Icons_*/Arch_Databases/64/Arch_Amazon-DynamoDB_64.svg",
    "ses": "Architecture-Service-Icons_*/Arch_Business-Applications/64/Arch_Amazon-Simple-Email-Service_64.svg",
    "sns": "Architecture-Service-Icons_*/Arch_Application-Integration/64/Arch_Amazon-Simple-Notification-Service_64.svg",
    "cloudwatch": "Architecture-Service-Icons_*/Arch_Management-Tools/64/Arch_Amazon-CloudWatch_64.svg",
    "rule": "Resource-Icons_*/Res_Application-Integration/Res_Amazon-EventBridge_Rule_48.svg",
    "topic": "Resource-Icons_*/Res_Application-Integration/Res_Amazon-Simple-Notification-Service_Topic_48.svg",
    "queue": "Resource-Icons_*/Res_Application-Integration/Res_Amazon-Simple-Queue-Service_Queue_48.svg",
    "logs": "Resource-Icons_*/Res_Management-Governance/Res_Amazon-CloudWatch_Logs_48.svg",
    "parameter": "Resource-Icons_*/Res_Management-Governance/Res_AWS-Systems-Manager_Parameter-Store_48.svg",
    "user": "Resource-Icons_*/Res_General-Icons/Res_48_Light/Res_User_48_Light.svg",
    "users": "Resource-Icons_*/Res_General-Icons/Res_48_Light/Res_Users_48_Light.svg",
    "client": "Resource-Icons_*/Res_General-Icons/Res_48_Light/Res_Client_48_Light.svg",
    "mobile": "Resource-Icons_*/Res_General-Icons/Res_48_Light/Res_Mobile-client_48_Light.svg",
}

FIREBASE = os.path.join(os.path.dirname(os.path.dirname(diagrams.__file__)), "resources", "firebase", "grow", "messaging.png")

# Graphviz lays the nodes out around a placeholder image, one file per icon,
# so the SVG can say which icon each node is; embed() swaps in the real ones
placeholders = tempfile.mkdtemp()
for name in [*ICONS, "firebase"]:
    shutil.copy(FIREBASE, os.path.join(placeholders, name + ".png"))


def node(label, icon):
    return Custom(label, os.path.join(placeholders, icon + ".png"))


def icon_data(name):
    """The data URI of an icon, given its placeholder's name"""
    if name == "firebase":
        with open(FIREBASE, "rb") as f:
            return "data:image/png;base64," + base64.b64encode(f.read()).decode()
    if name not in ICONS:
        raise SystemExit(f"no icon for {name}")
    matches = glob.glob(os.path.join(args.icons, ICONS[name]))
    if len(matches) != 1:
        raise SystemExit(f"{ICONS[name]}: expected one file in {args.icons}, found {len(matches)}")
    with open(matches[0], "rb") as f:
        return "data:image/svg+xml;base64," + base64.b64encode(f.read()).decode()


def embed(svg_path):
    """Replaces each image file in the SVG with its icon, embedded"""
    with open(svg_path, encoding="utf-8") as f:
        svg = f.read()

    def replace(match):
        name = os.path.splitext(os.path.basename(match.group(2)))[0]
        return match.group(1) + '"' + icon_data(name) + '"'

    svg = re.sub(r'(xlink:href=|href=)"([^"]+\.png)"', replace, svg)
    left = re.findall(r'href="(?!data:|#)([^"]*)"', svg)
    if left:
        raise SystemExit(f"images still refer to files: {left}")
    with open(svg_path, "w", encoding="utf-8", newline="\n") as f:
        f.write(svg)


GREY, PINK, GREEN, RED = "#545B64", "#C7166F", "#1E8900", "#D13212"

graph_attr = {
    "fontsize": "26",
    "fontname": "Helvetica",
    "labelloc": "t",
    "pad": "0.3",
    "nodesep": "0.35",
    "ranksep": "1.3",
    "splines": "spline",
    "newrank": "true",
    "bgcolor": "white",
}
node_attr = {"fontsize": "13", "fontname": "Helvetica"}
edge_attr = {"fontsize": "12", "fontname": "Helvetica", "color": GREY, "penwidth": "1.3"}


def edge(text="", color=GREY, style="solid", **attrs):
    return Edge(label=text, color=color, fontcolor=color, style=style, **attrs)


with Diagram(
    "Dead Man's Handle",
    filename=args.output,
    outformat="svg",
    show=False,
    direction="LR",
    graph_attr=graph_attr,
    node_attr=node_attr,
    edge_attr=edge_attr,
):
    owner = node("Owner", "user")
    cli = node(
        "dmh CLI, with the\nowner's credentials:\nalso the document\n(encrypts, decrypts)\nand the API's throttle",
        "client",
    )

    with Cluster("AWS account (one region)"):
        with Cluster("Push (optional)"):
            alarmpush_fn = node("alarmpush", "lambda")
            platform = node("Platform application\n(FCM HTTP v1)", "sns")

        with Cluster("Check-in"):
            api = node("HTTP API\nPOST /checkin", "api-gateway")
            http_fn = node("http", "lambda")

        with Cluster("Daily run"):
            schedule = node("Schedule rule\n02:00 UTC", "rule")
            scheduled_fn = node("scheduled", "lambda")

        with Cluster("Document changes"):
            upload_rule = node("Rule: uploads\nto the bucket", "rule")
            docwatch_fn = node("docwatch", "lambda")

        with Cluster("Flood guard (optional)"):
            check_rule = node("Check rule\nevery N min, on\nwhile API closed", "rule")
            apiguard_fn = node("apiguard", "lambda")

        with Cluster("Config and state"):
            config = node("Config (SecureString),\nparameters for dmh", "parameter")
            state = node("State item", "dynamodb")
            password = node("Document password\n(SecureString,\nonly dmh)", "parameter")

        bucket = node("Document bucket\n(encrypted .dmh)", "s3")

        with Cluster("Email"):
            dns = node("Route 53\nDKIM, DMARC", "route-53")
            ses = node("SES", "ses")

        with Cluster("Monitoring"):
            dlq = node("Dead-letter\nqueue", "queue")
            alarms = node("CloudWatch alarms\nerrors, not run,\nDLQ, API 4xx, push,\nguard", "cloudwatch")
            topic = node("Alarm topic", "topic")
            logs = node("Logs\n(14 days)", "logs")

    fcm = node("Firebase Cloud\nMessaging", "firebase")
    inbox = node("Owner's inbox", "user")
    phone = node("Owner's phone\n(app)", "mobile")
    # How recipients open the document is in the label: a node for the tools
    # rearranges the whole layout
    recipients = node(
        "Recipients: open it with\nopen-document or\ndecrypt.html and the\npassword from the owner\n"
        "(or from AWS, with the\nDMH-<app>-Recipient\npolicy)",
        "users",
    )

    # Check-in
    owner >> edge("check in\n(x-api-key)", PINK) >> api >> http_fn
    http_fn >> edge("read config\n(cached 5 min)") >> config
    http_fn >> edge("reset timeout,\npush token") >> state

    # The CLI works on AWS directly, not through the Lambdas
    owner >> edge(color=PINK) >> cli
    cli >> edge("settings", PINK, "dashed") >> config
    cli >> edge("password", PINK, "dashed") >> password
    cli >> edge("check in", PINK, "dashed") >> state

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
    ses >> edge("document\n(encrypted)", GREEN) >> recipients
    ses >> edge("warnings, notices", GREEN) >> inbox

    # Monitoring
    [schedule, upload_rule] >> edge("failed delivery", RED, "dashed") >> dlq
    dlq >> alarms >> topic >> edge("alarm email", RED) >> inbox

    # Flood guard: the API 4xx alarm closes the API; the check reopens it
    # once the alarm clears
    topic >> edge("API 4xx alarm", RED, constraint="false") >> apiguard_fn
    apiguard_fn >> edge("close, reopen\n(throttle 0/0)", RED, constraint="false") >> api
    check_rule >> apiguard_fn
    apiguard_fn >> edge("API notices", constraint="false") >> ses

    # Push: owner notices and alarms also go to the app, through FCM. The
    # edges into the row don't move the rest of the layout.
    # Labelled once: more labels crowd the push cluster's title
    scheduled_fn >> edge("owner notices", PINK, constraint="false") >> platform
    [docwatch_fn, apiguard_fn] >> edge(color=PINK, constraint="false") >> platform
    topic >> edge("alarms", RED, constraint="false") >> alarmpush_fn
    alarmpush_fn >> platform >> fcm >> edge("push", PINK) >> phone

    # Layout only: SES in the column after the data stores, logs bottom left,
    # and the push row under the alarms and the topic
    bucket >> Edge(style="invis") >> ses
    logs >> Edge(style="invis") >> dlq
    dlq >> Edge(style="invis") >> alarmpush_fn


embed(args.output + ".svg")
shutil.rmtree(placeholders)
