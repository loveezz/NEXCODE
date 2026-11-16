export type User = {
  id: number;
  login: string;
  name: string;
  role: "student" | "teacher" | "admin";
  blocked: boolean;
  created_at: string;
  full_name: string;
  group_name: string;
  invite_code_id: number | null;
  last_login_at: string;
  last_login_ip: string;
};
export type StudentGroup = {
  id: number;
  name: string;
  invite_required: boolean;
  members?: number;
  codes?: number;
};
export type LoginOptions = {
  group_mode: "free" | "list";
  groups: StudentGroup[];
  semester_end: string;
};
export type LoginEvent = {
  id: number;
  user_id: number | null;
  full_name: string;
  group_name: string;
  ip: string;
  success: boolean;
  created_at: string;
};
export type Lab = {
  id: number;
  title: string;
  description: string;
  position: number;
  source: string;
};
export type Exercise = {
  id: number;
  lab_id: number;
  title: string;
  statement: string;
  template: string;
  tests?: string;
  timeout_ms: number;
  memory_mb: number;
  difficulty: string;
  position: number;
  status: string;
  test_count: number;
};
export type Catalog = { labs: Lab[]; exercises: Exercise[] };
export type Diagnostic = {
  code: string;
  message: string;
  location: { row: number; column: number };
};
export type TestResult = {
  stdin: string;
  stdout: string;
  stderr: string;
  exit_code: number;
  duration_ms: number;
  timed_out: boolean;
  output_limit: boolean;
  passed: boolean;
};
// Expected stdout and actual stdout are named explicitly in the report wire format.
export type Result = {
  stdin: string;
  expected: string;
  actual: string;
  stderr: string;
  exit_code: number;
  duration_ms: number;
  timed_out: boolean;
  output_limit: boolean;
  passed: boolean;
};
export type Report = {
  results?: Result[];
  lint?: Diagnostic[];
  passed?: number;
  total?: number;
  error?: string;
};
export type Submission = {
  id: number;
  user_id: number;
  exercise_id: number;
  code: string;
  mode: string;
  status: string;
  report: Report;
  created_at: string;
  title: string;
  login?: string;
};
export type Student = {
  user: User;
  solved: number;
  total: number;
  progress: { exercise_id: number; title: string; status: string }[];
};
