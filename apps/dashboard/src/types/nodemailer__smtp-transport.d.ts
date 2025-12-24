declare module "nodemailer/lib/smtp-transport" {
  export type Options = {
    host?: string;
    port?: number;
    secure?: boolean;
    auth?: { user?: string; pass?: string };
    connectionTimeout?: number;
    greetingTimeout?: number;
    socketTimeout?: number;
  };
  export default class SMTPTransport {
    constructor(options: Options);
  }
}
