declare module "nodemailer" {
  export type Transporter = {
    sendMail: (options: { from?: string; to: string; subject: string; text?: string; html?: string }) => Promise<any>;
  };
  export function createTransport(options: any): Transporter;
}
