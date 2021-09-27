import { events, Event, Job, ConcurrentGroup, SerialGroup } from "@brigadecore/brigadier"

const releaseTagRegex = /^refs\/tags\/(v[0-9]+(?:\.[0-9]+)*(?:\-.+)?)$/

const goImg = "brigadecore/go-tools:v0.3.0"
const kanikoImg = "brigadecore/kaniko:v0.2.0"
const helmImg = "brigadecore/helm-tools:v0.4.0"
const localPath = "/workspaces/brigade-slack-gateway"

// MakeTargetJob is just a job wrapper around a make target.
class MakeTargetJob extends Job {
  constructor(target: string, img: string, event: Event, env?: {[key: string]: string}) {
    super(target, img, event)
    this.primaryContainer.sourceMountPath = localPath
    this.primaryContainer.workingDirectory = localPath
    this.primaryContainer.environment = env || {}
    this.primaryContainer.environment["SKIP_DOCKER"] = "true"
    if (event.worker?.git?.ref) {
      const matchStr = event.worker.git.ref.match(releaseTagRegex)
      if (matchStr) {
        this.primaryContainer.environment["VERSION"] = Array.from(matchStr)[1] as string
      }
    }
    this.primaryContainer.command = [ "make" ]
    this.primaryContainer.arguments = [ target ]
  }
}

// PushImageJob is a specialized job type for publishing Docker images.
class PushImageJob extends MakeTargetJob {
  constructor(target: string, event: Event) {
    super(target, kanikoImg, event, {
      "DOCKER_ORG": event.project.secrets.dockerhubOrg,
      "DOCKER_USERNAME": event.project.secrets.dockerhubUsername,
      "DOCKER_PASSWORD": event.project.secrets.dockerhubPassword
    })
  }
}

// A map of all jobs. When a check_run:rerequested event wants to re-run a
// single job, this allows us to easily find that job by name.
const jobs: {[key: string]: (event: Event) => Job } = {}

// Basic tests:

const testUnitJobName = "test-unit"
const testUnitJob = (event: Event) => {
  return new MakeTargetJob(testUnitJobName, goImg, event)
}
jobs[testUnitJobName] = testUnitJob

const lintJobName = "lint"
const lintJob = (event: Event) => {
  return new MakeTargetJob(lintJobName, goImg, event)
}
jobs[lintJobName] = lintJob

const lintChartJobName = "lint-chart"
const lintChartJob = (event: Event) => {
  return new MakeTargetJob(lintChartJobName, helmImg, event)
}
jobs[lintChartJobName] = lintChartJob

// Build / publish stuff:

const buildReceiverJobName = "build-receiver"
const buildReceiverJob = (event: Event) => {
  return new MakeTargetJob(buildReceiverJobName, kanikoImg, event)
}
jobs[buildReceiverJobName] = buildReceiverJob

const pushReceiverJobName = "push-receiver"
const pushReceiverJob = (event: Event) => {
  return new PushImageJob(pushReceiverJobName, event)
}
jobs[pushReceiverJobName] = pushReceiverJob

const buildMonitorJobName = "build-monitor"
const buildMonitorJob = (event: Event) => {
  return new MakeTargetJob(buildMonitorJobName, kanikoImg, event)
}
jobs[buildMonitorJobName] = buildMonitorJob

const pushMonitorJobName = "push-monitor"
const pushMonitorJob = (event: Event) => {
  return new PushImageJob(pushMonitorJobName, event)
}
jobs[pushMonitorJobName] = pushMonitorJob

const publishChartJobName = "publish-chart"
const publishChartJob = (event: Event) => {
  return new MakeTargetJob(publishChartJobName, helmImg, event, {
    "HELM_REGISTRY": event.project.secrets.helmRegistry || "ghcr.io",
    "HELM_ORG": event.project.secrets.helmOrg,
    "HELM_USERNAME": event.project.secrets.helmUsername,
    "HELM_PASSWORD": event.project.secrets.helmPassword
  })
}
jobs[publishChartJobName] = publishChartJob

// Run the entire suite of tests WITHOUT publishing anything initially. If
// EVERYTHING passes AND this was a push (merge, presumably) to the main branch,
// then run jobs to publish "edge" images.
async function runSuite(event: Event): Promise<void> {
  await new SerialGroup(
    new ConcurrentGroup( // Basic tests
      testUnitJob(event),
      lintJob(event),
      lintChartJob(event),
    ),
    new ConcurrentGroup( // Build everything
      buildReceiverJob(event),
      buildMonitorJob(event)
    )
  ).run()
  if (event.worker?.git?.ref == "main") {
    // Push "edge" images
    await new ConcurrentGroup(
      pushReceiverJob(event),
      pushMonitorJob(event)
    ).run()
  }
}

// Either of these events should initiate execution of the entire test suite.
events.on("brigade.sh/github", "check_suite:requested", runSuite)
events.on("brigade.sh/github", "check_suite:rerequested", runSuite)

// This event indicates a specific job is to be re-run.
events.on("brigade.sh/github", "check_run:rerequested", async event => {
  const jobName = JSON.parse(event.payload).check_run.name
  const job = jobs[jobName]
  if (job) {
    await job(event).run()
    return
  }
  throw new Error(`No job found with name: ${jobName}`)
})

// Pushing new commits to any branch in github triggers a check suite. Such
// events are already handled above. Here we're only concerned with the case
// wherein a new TAG has been pushed-- and even then, we're only concerned with
// tags that look like a semantic version and indicate a formal release should
// be performed.
events.on("brigade.sh/github", "push", async event => {
  const matchStr = event.worker.git.ref.match(releaseTagRegex)
  if (matchStr) {
    // This is an official release with a semantically versioned tag
    await new SerialGroup(
      new ConcurrentGroup(
        pushReceiverJob(event),
        pushMonitorJob(event)
      ),
      // Chart publishing is deliberately run only after all image pushes above
      // have succeeded. We don't want any possibility of publishing a chart
      // that references images that failed to push (or simply haven't
      // finished pushing).
      publishChartJob(event)
    ).run()
  } else {
    console.log(`Ref ${event.worker.git.ref} does not match release tag regex (${releaseTagRegex}); not releasing.`)
  }
})

events.process()
