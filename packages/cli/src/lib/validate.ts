const APP_NAME_REGEX = /^[a-z][a-z0-9-]*$/;

export function validateAppName(appName: string): void {
  if (!appName || !APP_NAME_REGEX.test(appName)) {
    throw new Error(
      `Invalid app name "${appName}". Use lowercase letters, numbers, and hyphens.`,
    );
  }
}
